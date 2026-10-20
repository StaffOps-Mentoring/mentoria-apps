// api é o backend da loja de lab: devolve o próprio status e uma lista de
// produtos guardada num Postgres. Sem DB_URL ela sobe e diz que não tem banco;
// com DB_URL e o banco fora do ar na partida, ela SAI — é o comportamento que
// torna visível a diferença entre "o banco subiu" e "o banco aceita conexão".
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// versao vem do build (-ldflags "-X main.versao=..."); VERSAO no ambiente vence.
var versao = "dev"

const (
	saidaPorta = 2 // PORTA inválida
	saidaBanco = 4 // banco inalcançável na partida
)

type api struct {
	versao string
	pod    string
	banco  *pgxpool.Pool
	alvo   string // host:porta/banco, sem senha, para log e status
}

func main() {
	log.SetFlags(log.LstdFlags)

	porta := valorOu(os.Getenv("PORTA"), "8080")
	if n, err := strconv.Atoi(porta); err != nil || n < 1 || n > 65535 {
		log.Printf("erro: PORTA=%q não é uma porta entre 1 e 65535", porta)
		os.Exit(saidaPorta)
	}
	tentativas, err := strconv.Atoi(valorOu(os.Getenv("DB_TENTATIVAS"), "1"))
	if err != nil || tentativas < 1 {
		tentativas = 1
	}

	a := &api{versao: valorOu(os.Getenv("VERSAO"), versao)}
	a.pod, _ = os.Hostname()

	if dsn := os.Getenv("DB_URL"); dsn != "" {
		a.alvo = semSenha(dsn)
		if err := a.conectar(dsn, tentativas); err != nil {
			log.Printf("erro: banco inalcançável em %s depois de %d tentativa(s): %v", a.alvo, tentativas, err)
			os.Exit(saidaBanco)
		}
		defer a.banco.Close()
	}

	srv := &http.Server{Addr: ":" + porta, Handler: a.rotas(), ReadHeaderTimeout: 5 * time.Second}
	falhou := make(chan error, 1)
	go func() { falhou <- srv.ListenAndServe() }()
	log.Printf("api %s no pod %s, porta %s, banco: %s", a.versao, a.pod, porta, valorOu(a.alvo, "não configurado"))

	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, syscall.SIGTERM, syscall.SIGINT)
	select {
	case err := <-falhou:
		log.Printf("erro: não consegui escutar: %v", err)
		os.Exit(1)
	case s := <-sinais:
		log.Printf("recebi %v, encerrando", s)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func (a *api) conectar(dsn string, tentativas int) error {
	var err error
	for i := 1; i <= tentativas; i++ {
		if err = a.abrir(dsn); err == nil {
			return a.prepararTabela()
		}
		log.Printf("banco %s: tentativa %d de %d falhou: %v", a.alvo, i, tentativas, err)
		if i < tentativas {
			time.Sleep(2 * time.Second)
		}
	}
	return err
}

func (a *api) abrir(dsn string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return err
	}
	a.banco = p
	return nil
}

func (a *api) prepararTabela() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := a.banco.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS produtos (
			id     serial PRIMARY KEY,
			nome   text UNIQUE NOT NULL,
			criado timestamptz NOT NULL DEFAULT now()
		);
		INSERT INTO produtos (nome)
			SELECT unnest(ARRAY['caneca', 'camiseta', 'adesivo'])
			WHERE NOT EXISTS (SELECT 1 FROM produtos);`)
	return err
}

func (a *api) rotas() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/status", a.status)
	m.HandleFunc("GET /api/produtos", a.listar)
	m.HandleFunc("POST /api/produtos", a.criar)
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	m.HandleFunc("GET /readyz", a.prontidao)
	return registrar(m)
}

func (a *api) estadoBanco(ctx context.Context) string {
	if a.banco == nil {
		return "não configurado"
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := a.banco.Ping(ctx); err != nil {
		return "erro: " + err.Error()
	}
	return "ok"
}

func (a *api) status(w http.ResponseWriter, r *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]string{
		"servico": "api", "versao": a.versao, "pod": a.pod, "banco": a.estadoBanco(r.Context()),
	})
}

// prontidao cai quando o banco configurado não responde: a api fica fora do
// balanceamento em vez de devolver 500 a quem chegar.
func (a *api) prontidao(w http.ResponseWriter, r *http.Request) {
	if e := a.estadoBanco(r.Context()); e != "ok" && e != "não configurado" {
		http.Error(w, "banco: "+e, http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "pronta")
}

func (a *api) listar(w http.ResponseWriter, r *http.Request) {
	if a.banco == nil {
		http.Error(w, "DB_URL não definida", http.StatusServiceUnavailable)
		return
	}
	linhas, err := a.banco.Query(r.Context(), `SELECT nome FROM produtos ORDER BY id`)
	if err != nil {
		log.Printf("listar: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer linhas.Close()
	nomes := []string{}
	for linhas.Next() {
		var n string
		if err := linhas.Scan(&n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nomes = append(nomes, n)
	}
	escreverJSON(w, http.StatusOK, map[string]any{"pod": a.pod, "produtos": nomes})
}

func (a *api) criar(w http.ResponseWriter, r *http.Request) {
	if a.banco == nil {
		http.Error(w, "DB_URL não definida", http.StatusServiceUnavailable)
		return
	}
	nome := r.URL.Query().Get("nome")
	if nome == "" || len(nome) > 64 {
		http.Error(w, "use ?nome= com até 64 caracteres", http.StatusBadRequest)
		return
	}
	_, err := a.banco.Exec(r.Context(), `INSERT INTO produtos (nome) VALUES ($1) ON CONFLICT (nome) DO NOTHING`, nome)
	if err != nil {
		log.Printf("criar: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	escreverJSON(w, http.StatusCreated, map[string]string{"criado": nome, "pod": a.pod})
}

// semSenha devolve host:porta/banco: a senha do DB_URL nunca vai para o log.
func semSenha(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return "(DB_URL ilegível)"
	}
	return u.Host + u.Path
}

type gravador struct {
	http.ResponseWriter
	codigo int
}

func (g *gravador) WriteHeader(c int) { g.codigo = c; g.ResponseWriter.WriteHeader(c) }

func registrar(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		g := &gravador{ResponseWriter: w, codigo: http.StatusOK}
		h.ServeHTTP(g, r)
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, g.codigo, time.Since(inicio).Round(time.Millisecond))
	})
}

func escreverJSON(w http.ResponseWriter, codigo int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(v)
}

func valorOu(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
