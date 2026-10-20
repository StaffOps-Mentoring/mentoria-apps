// cofre é um banco chave-valor mínimo, um arquivo por chave em DIR_DADOS.
// Serve para StatefulSet: cada réplica tem o próprio disco e o próprio nome,
// e GET / diz de qual pod a resposta veio.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

var versao = "dev"

var chaveValida = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func main() {
	log.SetFlags(log.LstdFlags)
	porta := valorOu(os.Getenv("PORTA"), "8080")
	if n, err := strconv.Atoi(porta); err != nil || n < 1 || n > 65535 {
		log.Printf("erro: PORTA=%q não é uma porta entre 1 e 65535", porta)
		os.Exit(2)
	}
	dir := valorOu(os.Getenv("DIR_DADOS"), "/dados")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("erro: não consegui usar %s: %v", dir, err)
		os.Exit(4)
	}
	pod, _ := os.Hostname()

	srv := &http.Server{Addr: ":" + porta, Handler: rotas(dir, pod), ReadHeaderTimeout: 5 * time.Second}
	falhou := make(chan error, 1)
	go func() { falhou <- srv.ListenAndServe() }()
	log.Printf("cofre %s no pod %s, porta %s, dados em %s", valorOu(os.Getenv("VERSAO"), versao), pod, porta, dir)

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

func rotas(dir, pod string) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintf(w, "cofre %s\n", pod) })
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	m.HandleFunc("PUT /chave/{k}", func(w http.ResponseWriter, r *http.Request) {
		k := r.PathValue("k")
		if !chaveValida.MatchString(k) {
			http.Error(w, "chave: letras, números, hífen ou sublinhado", http.StatusBadRequest)
			return
		}
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			http.Error(w, "valor acima de 64 KiB", http.StatusRequestEntityTooLarge)
			return
		}
		if err := os.WriteFile(filepath.Join(dir, k), b, 0o644); err != nil {
			log.Printf("gravar %s: %v", k, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("PUT %s (%d bytes)", k, len(b))
		fmt.Fprintf(w, "gravado %s no pod %s\n", k, pod)
	})
	m.HandleFunc("GET /chave/{k}", func(w http.ResponseWriter, r *http.Request) {
		k := r.PathValue("k")
		if !chaveValida.MatchString(k) {
			http.Error(w, "chave inválida", http.StatusBadRequest)
			return
		}
		b, err := os.ReadFile(filepath.Join(dir, k))
		if err != nil {
			http.Error(w, "não existe no pod "+pod, http.StatusNotFound)
			return
		}
		_, _ = w.Write(b)
	})
	return m
}

func valorOu(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
