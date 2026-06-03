package rproducer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// SupabaseURL retorna o endpoint REST do Supabase ("https://<ref>.supabase.co").
// Env: RPRODUCER_SUPABASE_URL OR SUPABASE_URL (compat com convenção).
func SupabaseURL() string {
	if v := strings.TrimSpace(os.Getenv("RPRODUCER_SUPABASE_URL")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("SUPABASE_URL"))
}

// SupabaseServiceKey retorna a service-role key (acesso total, bypassa RLS).
// Env: RPRODUCER_SUPABASE_SERVICE_KEY OR SUPABASE_SERVICE_ROLE_KEY.
func SupabaseServiceKey() string {
	if v := strings.TrimSpace(os.Getenv("RPRODUCER_SUPABASE_SERVICE_KEY")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
}

// SupabaseConfigured retorna true se temos URL+key suficientes pra rodar lookups.
func SupabaseConfigured() bool {
	return SupabaseURL() != "" && SupabaseServiceKey() != ""
}

// supabaseClient — singleton HTTP client com timeout reasonable.
var (
	supabaseClient   *http.Client
	supabaseClientMu sync.Once
)

func httpClient() *http.Client {
	supabaseClientMu.Do(func() {
		supabaseClient = &http.Client{Timeout: 10 * time.Second}
	})
	return supabaseClient
}

// supabaseGet — GET helper pra REST do Supabase. Retorna body raw.
func supabaseGet(ctx context.Context, path string) ([]byte, error) {
	url := strings.TrimRight(SupabaseURL(), "/") + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	key := SupabaseServiceKey()
	req.Header.Set("apikey", key)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ============================================================
// INSTANCE NAME CACHE — slug → Empresa.Nome
// ============================================================
//
// O whatsmeow Go expõe `Instance.Name` que pode estar normalizado (slug:
// "dennermaximillian") OU já com o nome real. O Agent espera o nome real
// ("Denner Maximillian") pra fazer `.eq('Nome', instance)` no Supabase.
//
// Bridge Node.js fazia esse mapping. Replicamos aqui: load Empresa table,
// build map[slug]→Nome, refresh a cada 5min.

var (
	instanceNameMap   = make(map[string]string)
	instanceNameMu    sync.RWMutex
	instanceNameOnce  sync.Once
	instanceNameReady = false
)

// slugify — replica `normalizeSlug` do bridge (lowercase + remove acentos +
// remove tudo que não é a-z0-9).
func slugify(s string) string {
	s = strings.ToLower(s)
	// Remoção de acentos comuns (NFD compose-only seria mais robusto, mas pra
	// compat exata com o bridge que usava string.normalize('NFD').replace(/[̀-ͯ]/g, '')
	// fazemos manual nos acentos PT-BR mais comuns).
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
		"é", "e", "ê", "e", "è", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c", "ñ", "n",
	)
	s = replacer.Replace(s)
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		}
	}
	return string(out)
}

// startInstanceNameCache — goroutine de reload a cada 5min. Idempotente.
func startInstanceNameCache() {
	instanceNameOnce.Do(func() {
		go func() {
			refreshInstanceNameCache(context.Background())
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				refreshInstanceNameCache(context.Background())
			}
		}()
	})
}

func refreshInstanceNameCache(ctx context.Context) {
	if !SupabaseConfigured() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// PostgREST: SELECT Nome FROM "Empresa" WHERE evogo_instance_id IS NOT NULL OR evolution_provider IS NOT NULL
	// (carregamos só empresas com instância)
	body, err := supabaseGet(ctx, `/rest/v1/Empresa?select=Nome&Nome=not.is.null&limit=10000`)
	if err != nil {
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return
	}
	newMap := make(map[string]string, len(rows))
	for _, r := range rows {
		nome, _ := r["Nome"].(string)
		nome = strings.TrimSpace(nome)
		if nome == "" {
			continue
		}
		slug := slugify(nome)
		if slug == "" {
			continue
		}
		newMap[slug] = nome
	}
	instanceNameMu.Lock()
	instanceNameMap = newMap
	instanceNameReady = true
	instanceNameMu.Unlock()
}

// ResolveInstanceName — recebe um nome (pode ser slug ou já o nome real) e
// retorna o Empresa.Nome real. Se não achar mapping, retorna o input intacto
// (fallback gracioso pra não bloquear publish).
func ResolveInstanceName(raw string) string {
	if !InstanceNameOverride() {
		return raw
	}
	startInstanceNameCache()
	instanceNameMu.RLock()
	defer instanceNameMu.RUnlock()
	if !instanceNameReady {
		return raw
	}
	// Primeiro tenta: input já é o nome real? (slug match contra ele mesmo)
	if real, ok := instanceNameMap[slugify(raw)]; ok {
		return real
	}
	return raw
}

// InstanceNameOverride — env var RPRODUCER_INSTANCE_NAME_OVERRIDE=true ativa.
func InstanceNameOverride() bool {
	return envOn("RPRODUCER_INSTANCE_NAME_OVERRIDE")
}

// ============================================================
// LID FALLBACK — Clientes.lid_jid lookup
// ============================================================
//
// Quando o whatsmeow não conseguiu resolver um @lid (Sender/Chat ainda @lid
// após o swap nativo), consultamos Supabase Clientes.lid_jid (que o bridge
// node.js + nossos backends populam ao longo do tempo).
//
// Cache em memória pra evitar query por mensagem.

var (
	lidCache   sync.Map // lidJid → realJid (string)
	lidCacheMu sync.Mutex
)

// LIDLookupEnabled — env var RPRODUCER_LID_LOOKUP=true ativa.
func LIDLookupEnabled() bool {
	return envOn("RPRODUCER_LID_LOOKUP")
}

// ResolveLidViaSupabase — recebe um JID @lid (ex: "X@lid") e tenta resolver
// pro real JID (@s.whatsapp.net) consultando Clientes.lid_jid.
// Retorna "" se não achou.
//
// Cache: hit é instantâneo. Miss é um round-trip HTTP (~30ms).
func ResolveLidViaSupabase(ctx context.Context, lidJid string) string {
	if !LIDLookupEnabled() || !SupabaseConfigured() {
		return ""
	}
	if !strings.Contains(lidJid, "@lid") {
		return ""
	}
	if cached, ok := lidCache.Load(lidJid); ok {
		if s, ok := cached.(string); ok {
			return s
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// PostgREST: GET /rest/v1/Clientes?select=number&lid_jid=eq.X@lid&number=not.is.null&limit=1
	path := `/rest/v1/Clientes?select=number&lid_jid=eq.` + lidJid + `&number=not.is.null&limit=1`
	body, err := supabaseGet(ctx, path)
	if err != nil {
		return ""
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) == 0 {
		// Cache "não achou" tb pra não martelar — TTL via sweep do dedupCache não cobre,
		// mas tudo bem: o map cresce devagar (só LIDs realmente desconhecidos).
		return ""
	}
	number, _ := rows[0]["number"].(string)
	if number == "" {
		return ""
	}
	// Limpar non-digits e construir JID
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)
	realJid := cleaned + "@s.whatsapp.net"
	lidCache.Store(lidJid, realJid)
	return realJid
}
