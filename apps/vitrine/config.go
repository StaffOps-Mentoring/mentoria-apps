package main

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

type config struct {
	nome         string
	versao       string
	porta        string
	portaAdmin   string
	apiURL       string
	dirUploads   string
	arqConfig    string
	arqToken     string
	token        string
	atrasoInicio time.Duration
	drenagem     time.Duration
	loja         map[string]string // toda variável LOJA_*, devolvida em /config
}

// Códigos de saída, documentados no README do repositório.
const (
	saidaPorta  = 2 // PORTA ou PORTA_ADMIN inválida
	saidaFalta  = 3 // variável obrigatória ausente
	saidaFormat = 5 // valor com formato inválido
)

func carregarConfig(getenv func(string) string, ambiente []string) (config, error) {
	c := config{
		nome:       getenv("LOJA_NOME"),
		versao:     valorOu(getenv("VERSAO"), versao),
		porta:      valorOu(getenv("PORTA"), "8080"),
		portaAdmin: getenv("PORTA_ADMIN"),
		apiURL:     getenv("API_URL"),
		dirUploads: valorOu(getenv("DIR_UPLOADS"), "/tmp/uploads"),
		arqConfig:  valorOu(getenv("ARQUIVO_CONFIG"), "/config/vitrine.conf"),
		arqToken:   getenv("ARQUIVO_TOKEN"),
		token:      getenv("TOKEN"),
		loja:       map[string]string{},
	}
	for _, kv := range ambiente {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(k, "LOJA_") {
			c.loja[k] = v
		}
	}
	if c.portaAdmin == "" {
		c.portaAdmin = "9090"
	} else if c.portaAdmin == "0" {
		c.portaAdmin = ""
	}

	if c.nome == "" {
		return c, errConfig(saidaFalta, "LOJA_NOME não definida: a vitrine não sabe de que loja é")
	}
	for _, p := range []struct{ nome, valor string }{{"PORTA", c.porta}, {"PORTA_ADMIN", c.portaAdmin}} {
		if p.valor == "" {
			continue
		}
		if n, err := strconv.Atoi(p.valor); err != nil || n < 1 || n > 65535 {
			return c, errConfig(saidaPorta, "%s=%q não é uma porta entre 1 e 65535", p.nome, p.valor)
		}
	}
	if c.porta == c.portaAdmin {
		return c, errConfig(saidaPorta, "PORTA e PORTA_ADMIN são a mesma (%s)", c.porta)
	}

	var err error
	if c.atrasoInicio, err = duracaoOuFaixa(getenv("ATRASO_INICIO")); err != nil {
		return c, errConfig(saidaFormat, "ATRASO_INICIO=%q: %v", getenv("ATRASO_INICIO"), err)
	}
	if c.drenagem, err = duracaoOuFaixa(getenv("DRENAGEM")); err != nil {
		return c, errConfig(saidaFormat, "DRENAGEM=%q: %v", getenv("DRENAGEM"), err)
	}
	return c, nil
}

// duracaoOuFaixa aceita "", "20" (segundos), "20s", "1m" ou uma faixa "20-40",
// sorteada a cada partida: é o que faz o tempo de subida não ser decorável.
func duracaoOuFaixa(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if de, ate, ok := strings.Cut(s, "-"); ok {
		a, err1 := segundos(de)
		b, err2 := segundos(ate)
		if err1 != nil || err2 != nil || b < a {
			return 0, errFaixa
		}
		return a + time.Duration(rand.Int64N(int64(b-a)+1)), nil
	}
	return segundos(s)
}

type erroTexto string

func (e erroTexto) Error() string { return string(e) }

const errFaixa = erroTexto("use segundos (20), duração (20s) ou faixa (20-40)")

func segundos(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil && n >= 0 {
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, errFaixa
	}
	return d, nil
}
