package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGravarELer(t *testing.T) {
	h := rotas(t.TempDir(), "cofre-0")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("PUT", "/chave/minha", strings.NewReader("valor")))
	if rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/chave/minha", nil))
	if rec.Code != 200 || rec.Body.String() != "valor" {
		t.Errorf("GET: %d %q", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/chave/outra", nil))
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "cofre-0") {
		t.Errorf("GET inexistente: %d %q", rec.Code, rec.Body)
	}
}
