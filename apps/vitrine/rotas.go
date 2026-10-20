package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type vitrine struct {
	cfg     config
	pod     string
	api     *httputil.ReverseProxy
	pronta  atomic.Bool
	travada atomic.Bool

	// taxa de erro em milésimos e até quando ela vale (unix nano)
	taxaErro  atomic.Int64
	erroAte   atomic.Int64
	memoriaMu sync.Mutex // um pico por vez

	metricasMu sync.Mutex
	pedidos    map[string]uint64 // por código de status
}

func novaVitrine(cfg config) *vitrine {
	pod, _ := os.Hostname()
	v := &vitrine{cfg: cfg, pod: pod, pedidos: map[string]uint64{}}
	if cfg.apiURL != "" {
		if u, err := url.Parse(cfg.apiURL); err == nil && u.Host != "" {
			v.api = httputil.NewSingleHostReverseProxy(u)
			v.api.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("api inalcançável em %s: %v", u.Host, err)
				http.Error(w, "api inalcançável: "+err.Error(), http.StatusBadGateway)
			}
		} else {
			log.Printf("aviso: API_URL=%q não é uma URL com host; /api/ vai responder 503", cfg.apiURL)
		}
	}
	return v
}

func (v *vitrine) rotas() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /{$}", v.inicio)
	m.HandleFunc("GET /version", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, v.cfg.versao) })
	m.HandleFunc("GET /hostname", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, v.pod) })
	m.HandleFunc("GET /healthz", v.saude)
	m.HandleFunc("GET /readyz", v.prontidao)
	m.HandleFunc("GET /config", v.config)
	m.HandleFunc("GET /segredo", v.segredo)
	m.HandleFunc("POST /upload", v.gravarUpload)
	m.HandleFunc("GET /upload/{nome}", v.lerUpload)
	m.HandleFunc("GET /metrics", v.metricas)
	m.HandleFunc("/api/", v.repassarAPI)
	return v.registrar(m)
}

func (v *vitrine) rotasAdmin() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /admin/travar", func(w http.ResponseWriter, _ *http.Request) {
		v.travada.Store(true)
		log.Printf("admin: travada — /healthz passa a responder 500")
		fmt.Fprintln(w, "travada")
	})
	m.HandleFunc("POST /admin/destravar", func(w http.ResponseWriter, _ *http.Request) {
		v.travada.Store(false)
		log.Printf("admin: destravada")
		fmt.Fprintln(w, "destravada")
	})
	m.HandleFunc("POST /admin/memoria", v.picoMemoria)
	m.HandleFunc("POST /admin/erros", v.provocarErros)
	return v.registrar(m)
}

func (v *vitrine) inicio(w http.ResponseWriter, r *http.Request) {
	if v.sortearErro() {
		http.Error(w, "erro provocado", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Loja %s — vitrine %s — pod %s\n", v.cfg.nome, v.cfg.versao, v.pod)
}

func (v *vitrine) saude(w http.ResponseWriter, _ *http.Request) {
	if v.travada.Load() {
		http.Error(w, "travada", http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, "ok")
}

func (v *vitrine) prontidao(w http.ResponseWriter, _ *http.Request) {
	if !v.pronta.Load() {
		http.Error(w, "ainda não", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "pronta")
}

// config devolve as variáveis LOJA_* lidas na partida e o arquivo de config
// lido AGORA: a primeira parte só muda com um pod novo, a segunda muda sozinha
// quando o volume muda.
func (v *vitrine) config(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]any{"ambiente": v.cfg.loja, "arquivo": v.cfg.arqConfig}
	if b, err := os.ReadFile(v.cfg.arqConfig); err == nil {
		resp["conteudo"] = string(b)
	} else {
		resp["erro"] = err.Error()
	}
	escreverJSON(w, http.StatusOK, resp)
}

// segredo nunca devolve o valor: só o que se precisa para conferir que ele
// chegou inteiro (tamanho, hash, quebra de linha no fim).
func (v *vitrine) segredo(w http.ResponseWriter, _ *http.Request) {
	valor, origem := v.cfg.token, "TOKEN"
	if v.cfg.arqToken != "" {
		b, err := os.ReadFile(v.cfg.arqToken)
		if err != nil {
			escreverJSON(w, http.StatusServiceUnavailable, map[string]any{"origem": v.cfg.arqToken, "erro": err.Error()})
			return
		}
		valor, origem = string(b), v.cfg.arqToken
	}
	if valor == "" {
		escreverJSON(w, http.StatusNotFound, map[string]any{"origem": origem, "erro": "vazio"})
		return
	}
	h := sha256.Sum256([]byte(valor))
	escreverJSON(w, http.StatusOK, map[string]any{
		"origem":          origem,
		"tamanho":         len(valor),
		"sha256":          hex.EncodeToString(h[:]),
		"quebra_de_linha": strings.HasSuffix(valor, "\n"),
	})
}

var nomeValido = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func (v *vitrine) gravarUpload(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	if !nomeValido.MatchString(nome) || strings.HasPrefix(nome, ".") {
		http.Error(w, "use ?nome= com letras, números, ponto, hífen ou sublinhado", http.StatusBadRequest)
		return
	}
	corpo, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "corpo acima de 1 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	if err := os.MkdirAll(v.cfg.dirUploads, 0o755); err != nil {
		log.Printf("upload: não consegui criar %s: %v", v.cfg.dirUploads, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	caminho := filepath.Join(v.cfg.dirUploads, nome)
	if err := os.WriteFile(caminho, corpo, 0o644); err != nil {
		log.Printf("upload: não consegui gravar %s: %v", caminho, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "gravado %s (%d bytes) no pod %s\n", nome, len(corpo), v.pod)
}

func (v *vitrine) lerUpload(w http.ResponseWriter, r *http.Request) {
	nome := r.PathValue("nome")
	if !nomeValido.MatchString(nome) || strings.HasPrefix(nome, ".") {
		http.Error(w, "nome inválido", http.StatusBadRequest)
		return
	}
	b, err := os.ReadFile(filepath.Join(v.cfg.dirUploads, nome))
	if err != nil {
		http.Error(w, "não encontrado neste pod ("+v.pod+")", http.StatusNotFound)
		return
	}
	_, _ = w.Write(b)
}

func (v *vitrine) repassarAPI(w http.ResponseWriter, r *http.Request) {
	if v.api == nil {
		http.Error(w, "API_URL não definida", http.StatusServiceUnavailable)
		return
	}
	v.api.ServeHTTP(w, r)
}

// picoMemoria aloca e toca mb MiB por alguns segundos e devolve: um pico que
// o kubectl top, que amostra a cada ~15 s, quase nunca vê.
func (v *vitrine) picoMemoria(w http.ResponseWriter, r *http.Request) {
	mb, err1 := strconv.Atoi(r.URL.Query().Get("mb"))
	seg, err2 := strconv.Atoi(valorOu(r.URL.Query().Get("segundos"), "3"))
	if err1 != nil || err2 != nil || mb < 1 || mb > 4096 || seg < 1 || seg > 300 {
		http.Error(w, "use ?mb=1..4096&segundos=1..300", http.StatusBadRequest)
		return
	}
	if !v.memoriaMu.TryLock() {
		http.Error(w, "já há um pico em andamento", http.StatusConflict)
		return
	}
	log.Printf("admin: pico de %d MiB por %ds", mb, seg)
	go func() {
		defer v.memoriaMu.Unlock()
		bloco := make([]byte, mb<<20)
		for i := 0; i < len(bloco); i += 4096 {
			bloco[i] = 1
		}
		time.Sleep(time.Duration(seg) * time.Second)
		bloco = nil
		debug.FreeOSMemory()
		log.Printf("admin: pico de %d MiB liberado", mb)
	}()
	fmt.Fprintf(w, "pico de %d MiB por %ds\n", mb, seg)
}

func (v *vitrine) provocarErros(w http.ResponseWriter, r *http.Request) {
	taxa, err1 := strconv.ParseFloat(r.URL.Query().Get("taxa"), 64)
	seg, err2 := strconv.Atoi(valorOu(r.URL.Query().Get("segundos"), "60"))
	if err1 != nil || err2 != nil || taxa < 0 || taxa > 1 || seg < 1 || seg > 3600 {
		http.Error(w, "use ?taxa=0..1&segundos=1..3600", http.StatusBadRequest)
		return
	}
	v.taxaErro.Store(int64(taxa * 1000))
	v.erroAte.Store(time.Now().Add(time.Duration(seg) * time.Second).UnixNano())
	log.Printf("admin: %.0f%% de erro em / por %ds", taxa*100, seg)
	fmt.Fprintf(w, "%.0f%% de erro em / por %ds\n", taxa*100, seg)
}

func (v *vitrine) sortearErro() bool {
	if time.Now().UnixNano() > v.erroAte.Load() {
		return false
	}
	return rand.Int64N(1000) < v.taxaErro.Load()
}

// metricas escreve o formato de texto do Prometheus à mão: é pouco, e mostra
// que não há mágica no /metrics.
func (v *vitrine) metricas(w http.ResponseWriter, _ *http.Request) {
	v.metricasMu.Lock()
	codigos := make([]string, 0, len(v.pedidos))
	for c := range v.pedidos {
		codigos = append(codigos, c)
	}
	sort.Strings(codigos)
	var b strings.Builder
	b.WriteString("# HELP vitrine_pedidos_total Pedidos atendidos na porta principal, por código.\n")
	b.WriteString("# TYPE vitrine_pedidos_total counter\n")
	for _, c := range codigos {
		fmt.Fprintf(&b, "vitrine_pedidos_total{codigo=%q} %d\n", c, v.pedidos[c])
	}
	v.metricasMu.Unlock()
	pronta := 0
	if v.pronta.Load() {
		pronta = 1
	}
	b.WriteString("# HELP vitrine_pronta 1 quando a vitrine terminou de subir.\n")
	b.WriteString("# TYPE vitrine_pronta gauge\n")
	fmt.Fprintf(&b, "vitrine_pronta %d\n", pronta)
	b.WriteString("# HELP vitrine_info Versão em execução.\n")
	b.WriteString("# TYPE vitrine_info gauge\n")
	fmt.Fprintf(&b, "vitrine_info{versao=%q} 1\n", v.cfg.versao)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = io.WriteString(w, b.String())
}

type gravador struct {
	http.ResponseWriter
	codigo int
}

func (g *gravador) WriteHeader(c int) { g.codigo = c; g.ResponseWriter.WriteHeader(c) }

// registrar escreve uma linha por pedido e conta por código. /metrics, /healthz
// e /readyz não entram no log: são chamados a cada poucos segundos e afogariam
// a linha que interessa a quem depura.
func (v *vitrine) registrar(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		g := &gravador{ResponseWriter: w, codigo: http.StatusOK}
		h.ServeHTTP(g, r)
		if strings.HasPrefix(r.URL.Path, "/admin/") {
			return
		}
		v.metricasMu.Lock()
		v.pedidos[strconv.Itoa(g.codigo)]++
		v.metricasMu.Unlock()
		switch r.URL.Path {
		case "/metrics", "/healthz", "/readyz":
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
