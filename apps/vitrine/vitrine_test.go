package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ambiente(kv ...string) (func(string) string, []string) {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	var lista []string
	for k, v := range m {
		lista = append(lista, k+"="+v)
	}
	return func(k string) string { return m[k] }, lista
}

func vitrineDeTeste(t *testing.T, kv ...string) *vitrine {
	t.Helper()
	cfg, err := carregarConfig(ambiente(append([]string{"LOJA_NOME", "teste", "DIR_UPLOADS", t.TempDir()}, kv...)...))
	if err != nil {
		t.Fatal(err)
	}
	return novaVitrine(cfg)
}

func pedir(t *testing.T, h http.Handler, metodo, caminho, corpo string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(metodo, caminho, strings.NewReader(corpo)))
	b, _ := io.ReadAll(rec.Body)
	return rec.Code, string(b)
}

func TestCodigosDeSaida(t *testing.T) {
	casos := []struct {
		nome   string
		kv     []string
		codigo int
	}{
		{"sem LOJA_NOME", nil, saidaFalta},
		{"produção sem segredo", []string{"LOJA_NOME", "x", "AMBIENTE", "producao", "API_URL", "http://api"}, saidaFalta},
		{"produção sem api", []string{"LOJA_NOME", "x", "AMBIENTE", "producao", "SEGREDO_SESSAO", "s"}, saidaFalta},
		{"porta texto", []string{"LOJA_NOME", "x", "PORTA", "http"}, saidaPorta},
		{"portas iguais", []string{"LOJA_NOME", "x", "PORTA", "9090"}, saidaPorta},
		{"atraso inválido", []string{"LOJA_NOME", "x", "ATRASO_INICIO", "40-20"}, saidaFormat},
	}
	for _, c := range casos {
		_, err := carregarConfig(ambiente(c.kv...))
		var ec erroConfig
		if !errors.As(err, &ec) || ec.codigo != c.codigo {
			t.Errorf("%s: queria código %d, veio %v", c.nome, c.codigo, err)
		}
	}
}

func TestFaixaDeAtraso(t *testing.T) {
	for i := 0; i < 50; i++ {
		d, err := duracaoOuFaixa("20-40")
		if err != nil || d < 20*time.Second || d > 40*time.Second {
			t.Fatalf("fora da faixa: %v %v", d, err)
		}
	}
}

func TestVersaoEProntidao(t *testing.T) {
	v := vitrineDeTeste(t, "VERSAO", "1.2.3")
	h := v.rotas()
	if c, b := pedir(t, h, "GET", "/version", ""); c != 200 || b != "1.2.3\n" {
		t.Errorf("/version: %d %q", c, b)
	}
	if c, _ := pedir(t, h, "GET", "/readyz", ""); c != 503 {
		t.Errorf("/readyz antes de subir: %d", c)
	}
	v.iniciar(0)
	if c, _ := pedir(t, h, "GET", "/readyz", ""); c != 200 {
		t.Errorf("/readyz depois de subir: %d", c)
	}
}

func TestTravar(t *testing.T) {
	v := vitrineDeTeste(t)
	pedir(t, v.rotasAdmin(), "POST", "/admin/travar", "")
	if c, _ := pedir(t, v.rotas(), "GET", "/healthz", ""); c != 500 {
		t.Errorf("/healthz travada: %d", c)
	}
}

func TestUpload(t *testing.T) {
	h := vitrineDeTeste(t).rotas()
	if c, _ := pedir(t, h, "POST", "/upload?nome=../x", "a"); c != 400 {
		t.Errorf("nome com barra aceito: %d", c)
	}
	if c, b := pedir(t, h, "POST", "/upload?nome=tok", "abc"); c != 200 {
		t.Fatalf("gravar: %d %s", c, b)
	}
	if c, b := pedir(t, h, "GET", "/upload/tok", ""); c != 200 || b != "abc" {
		t.Errorf("ler: %d %q", c, b)
	}
}

func TestSegredoMostraQuebraDeLinha(t *testing.T) {
	h := vitrineDeTeste(t, "TOKEN", "s3cr3t\n").rotas()
	c, b := pedir(t, h, "GET", "/segredo", "")
	if c != 200 || !strings.Contains(b, `"quebra_de_linha":true`) || strings.Contains(b, "s3cr3t") {
		t.Errorf("/segredo: %d %s", c, b)
	}
}

func TestMetricasContam(t *testing.T) {
	h := vitrineDeTeste(t).rotas()
	pedir(t, h, "GET", "/", "")
	pedir(t, h, "GET", "/nao-existe", "")
	_, b := pedir(t, h, "GET", "/metrics", "")
	for _, linha := range []string{`vitrine_pedidos_total{codigo="200"} 1`, `vitrine_pedidos_total{codigo="404"} 1`} {
		if !strings.Contains(b, linha) {
			t.Errorf("faltou %q em:\n%s", linha, b)
		}
	}
}

func TestErrosProvocados(t *testing.T) {
	v := vitrineDeTeste(t)
	pedir(t, v.rotasAdmin(), "POST", "/admin/erros?taxa=1&segundos=60", "")
	if c, _ := pedir(t, v.rotas(), "GET", "/", ""); c != 500 {
		t.Errorf("taxa 1 respondeu %d", c)
	}
}

func TestSemAPI(t *testing.T) {
	if c, _ := pedir(t, vitrineDeTeste(t).rotas(), "GET", "/api/status", ""); c != 503 {
		t.Errorf("sem API_URL: %d", c)
	}
}

func TestProducaoCompleta(t *testing.T) {
	_, err := carregarConfig(ambiente("LOJA_NOME", "x", "AMBIENTE", "producao", "SEGREDO_SESSAO", "s", "API_URL", "http://api"))
	if err != nil {
		t.Errorf("produção completa recusada: %v", err)
	}
}
