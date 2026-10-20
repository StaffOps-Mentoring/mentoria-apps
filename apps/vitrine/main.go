// vitrine é o front da loja de lab. Responde HTTP, repassa /api/ para a api,
// guarda uploads, lê config e token, expõe métricas e tem controles de falha
// numa porta de administração separada, que é por onde o verify provoca os
// cenários (travar, pico de memória, taxa de erro).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// versao vem do build (-ldflags "-X main.versao=..."); VERSAO no ambiente vence.
var versao = "dev"

func main() {
	log.SetFlags(log.LstdFlags)

	cfg, err := carregarConfig(os.Getenv, os.Environ())
	if err != nil {
		var ec erroConfig
		if errors.As(err, &ec) {
			log.Printf("erro: %v", ec.msg)
			os.Exit(ec.codigo)
		}
		log.Printf("erro: %v", err)
		os.Exit(1)
	}

	v := novaVitrine(cfg)

	principal := &http.Server{Addr: ":" + cfg.porta, Handler: v.rotas(), ReadHeaderTimeout: 5 * time.Second}
	var admin *http.Server
	if cfg.portaAdmin != "" {
		admin = &http.Server{Addr: ":" + cfg.portaAdmin, Handler: v.rotasAdmin(), ReadHeaderTimeout: 5 * time.Second}
	}

	falhou := make(chan error, 2)
	go func() { falhou <- principal.ListenAndServe() }()
	if admin != nil {
		go func() { falhou <- admin.ListenAndServe() }()
	}

	log.Printf("vitrine %s da loja %q no pod %s, porta %s (admin: %s)",
		cfg.versao, cfg.nome, v.pod, cfg.porta, valorOu(cfg.portaAdmin, "desligada"))
	go v.iniciar(cfg.atrasoInicio)

	sinais := make(chan os.Signal, 1)
	signal.Notify(sinais, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-falhou:
		log.Printf("erro: não consegui escutar: %v", err)
		os.Exit(1)
	case s := <-sinais:
		log.Printf("recebi %v: fora do balanceamento, drenando por %s", s, cfg.drenagem)
		v.pronta.Store(false)
		time.Sleep(cfg.drenagem)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = principal.Shutdown(ctx)
		if admin != nil {
			_ = admin.Shutdown(ctx)
		}
		log.Printf("encerrada")
	}
}

func valorOu(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}

// iniciar simula uma app que demora para ficar pronta (ATRASO_INICIO).
func (v *vitrine) iniciar(atraso time.Duration) {
	if atraso > 0 {
		log.Printf("aquecendo: pronta em %s", atraso)
		time.Sleep(atraso)
	}
	v.pronta.Store(true)
	log.Printf("pronta")
}

// erroConfig carrega o código de saída: quem depura um container lê o código
// e a última linha de log, e os dois têm de apontar a mesma causa.
type erroConfig struct {
	codigo int
	msg    string
}

func (e erroConfig) Error() string { return e.msg }

func errConfig(codigo int, formato string, a ...any) error {
	return erroConfig{codigo: codigo, msg: fmt.Sprintf(formato, a...)}
}
