# RProducer changes

Catálogo de TODA modificação aplicada em cima do upstream Evolution Go. Procure por `[RPRODUCER]` no código pra achar cada change inline.

## Convenção

Todo trecho de código adicionado/modificado tem comentário com prefix `[RPRODUCER]`. Isso permite:
- `grep -rn "\[RPRODUCER\]" pkg/` lista tudo
- Rebase com upstream: conflitos sinalizam os trechos a re-aplicar

## v0.7.1-rwa-1 (base: upstream tag 0.7.1)

### 1. `safeConn` wrapper — race condition em WebSocket writes

**Arquivo:** [`pkg/events/websocket/websocket_producer.go`](./pkg/events/websocket/websocket_producer.go)

**Problema:** `gorilla/websocket` não é thread-safe pra writes concorrentes ([doc oficial](https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency)). O producer chama `WriteJSON` de várias goroutines (eventos whatsmeow, broadcast, etc) sem mutex por-conn — em prod isso panic ou corrompe a conexão.

**Fix:** introduzido `safeConn struct` que encapsula `*websocket.Conn` com `sync.Mutex`. Métodos:
- `WriteJSON(v)` — `mu.Lock(); defer mu.Unlock()` antes do write
- `ReadMessage()` — pass-through (gorilla aceita 1 read + 1 write concorrentes)
- `Close()` — `mu.Lock()` pra não fazer race com WriteJSON em voo

Trocado o tipo do `clients map[string]*websocket.Conn` → `map[string]*safeConn` (idem broadcast slice). `ServeWs` wrappa com `newSafeConn()` antes de adicionar ao mapa.

**Símbolos novos no binário:**
- `newSafeConn`, `safeConn.WriteJSON`, `safeConn.ReadMessage`, `safeConn.Close`

### 2. `POST /chat/resolveLid` — LID → PN resolver

**Arquivos:**
- [`pkg/chat/service/chat_service.go`](./pkg/chat/service/chat_service.go) — método `ResolveLid` + structs `ResolveLidStruct`/`ResolveLidResponse`
- [`pkg/chat/handler/chat_handler.go`](./pkg/chat/handler/chat_handler.go) — handler `ResolveLid`
- [`pkg/routes/routes.go`](./pkg/routes/routes.go) — `routes.POST("/resolveLid", r.chatHandler.ResolveLid)`

**Motivo:** o whatsmeow mantém um `LIDStore` interno que mapeia LID (`@lid`) ↔ phone-number JID (`@s.whatsapp.net`). O upstream não expõe esse store via HTTP. Sem isso, conversas históricas que vieram apenas como `@lid` (típico de WhatsApp Business com privacidade ligada) ficavam órfãs no CRM — não havia jeito de descobrir o número real.

**Uso:**
```bash
# Request
curl -X POST https://evolution-go.host/chat/resolveLid \
  -H "Content-Type: application/json" \
  -H "apikey: $INSTANCE_TOKEN" \
  -d '{"lid": "116419450638400@lid"}'

# Response (mapping conhecido)
{
  "message": "success",
  "data": {
    "lid": "116419450638400@lid",
    "pn":  "553184041001@s.whatsapp.net",
    "found": true
  }
}

# Response (LID desconhecido)
{ "message": "success", "data": { "lid": "X@lid", "pn": "", "found": false } }
```

**Implementação:** `client.Store.LIDs.GetPNForLID(ctx, lidJID)` com timeout de 5s. Idempotente, sem ida pra rede (lookup local na SQLite/Postgres do whatsmeow).

### 3. Branding

**Arquivos:** `README.md`, `RPRODUCER-CHANGES.md` (este), `UPSTREAM-SYNC.md`, `TRADEMARKS.md`.

Reescritos pra refletir que é um fork RProducer, sem usar marca "Evolution" no produto. LICENSE Apache 2.0 mantida (herdada). Trademark notice mantida (citamos upstream em "based on Evolution Go").

---

## Próximas modificações planejadas

(Nada por enquanto — patches só conforme necessidade real.)
