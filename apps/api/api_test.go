package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSemBanco(t *testing.T) {
	h := (&api{versao: "1.0.0", pod: "p"}).rotas()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"banco":"não configurado"`) {
		t.Errorf("/api/status: %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/produtos", nil))
	if rec.Code != 503 {
		t.Errorf("/api/produtos sem banco: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/status", nil))
	if rec.Code != 404 {
		t.Errorf("/status sem o prefixo /api: %d", rec.Code)
	}
}

func TestSemSenha(t *testing.T) {
	if s := semSenha("postgres://loja:segredo@db:5432/loja?sslmode=disable"); s != "db:5432/loja" || strings.Contains(s, "segredo") {
		t.Errorf("semSenha: %q", s)
	}
}
