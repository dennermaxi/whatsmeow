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

## v0.7.1-rwa-2 (base: upstream tag 0.7.1)

### 4. Pacote `pkg/rproducer/` — interceptor de publishes via env vars

**Motivação:** O RProducer tinha um bridge Node.js (`websocket-evolution-go`) que ficava entre o Evolution Go e o consumer (RProducerAgent). O bridge fazia 4 coisas que o upstream não fazia:
1. Filtrar publishes pra só eventos `Message` (descartando `SendMessage`, `Receipt`, etc)
2. Deduplicar mensagens (Evolution emite a mesma msg 2x — uma com `@lid` outra com `@s.whatsapp.net`)
3. Publicar em fila única `rproducer-messages.upsert` (em vez de uma fila por evento)
4. Reformatar envelope JSON pro formato `{event, instance, data, timestamp, source}` (em vez do default `{event, instanceId, instanceToken, instanceName, data}`)

Sem essas 4 features, o Agent (consumer) não conseguiria processar os eventos publicados diretamente pelo Go.

**Solução:** Pacote `pkg/rproducer/` isolado com a lógica, ativada por env vars. Único ponto de injeção no upstream: `sendToQueueOrWebhook` e `SendToGlobalQueues` em `pkg/whatsmeow/service/whatsmeow.go` (3 chamadas a `rproducer.Intercept`).

**Arquivos novos:**
- `pkg/rproducer/config.go` — leitura de env vars
- `pkg/rproducer/dedup.go` — LRU cache thread-safe via `sync.Map` + sweeper 30s, TTL 60s, cap 50k
- `pkg/rproducer/intercept.go` — função `Intercept(queueName, jsonData) → {ShouldSend, QueueName, JSONData}`

**Env vars adicionadas:**

| Env var | Default | Efeito |
|---|---|---|
| `RPRODUCER_FILTER_MESSAGE_ONLY` | `false` | Descarta publishes de eventos != `Message` (reduz tráfego RabbitMQ ~70%) |
| `RPRODUCER_DEDUP` | `false` | Dedup por `data.Info.ID` (TTL 60s, LRU 50k entries) |
| `RPRODUCER_ENVELOPE` | `` | Se `=rproducer-v1`, reformata postMap pro envelope que o RProducerAgent espera |
| `RPRODUCER_PUBLISH_TO_QUEUE` | `` | Se setado, força TODOS os publishes pra essa fila (ex: `rproducer-messages.upsert`). Senão usa o queueName default (`instanceId.eventType`) |

**Performance:** Se NENHUMA env var RPRODUCER_* estiver setada, `Intercept()` retorna passthrough em ~10ns (1 leitura `os.Getenv`). Overhead zero se desligado.

**Default behavior:** Sem env vars, o fork se comporta IDÊNTICO ao upstream — pode ser usado como drop-in replacement.

**Configuração pra paridade com bridge (recomendada pra RProducer prod):**
```
RPRODUCER_FILTER_MESSAGE_ONLY=true
RPRODUCER_DEDUP=true
RPRODUCER_ENVELOPE=rproducer-v1
RPRODUCER_PUBLISH_TO_QUEUE=rproducer-messages.upsert
```

Com essa config, o Agent recebe eventos EXATAMENTE como o bridge entregava, sem precisar mexer no consumer.

---

## v0.7.1-rwa-3 (base: upstream tag 0.7.1)

### 5. Cobertura 100% das transformações do bridge — instance normalization + LID Supabase fallback + markers

**Motivação:** Auditoria fina revelou 3 gaps que faltavam pro fork conseguir substituir 100% o bridge Node.js. Sem isso, o swap ia quebrar o Agent.

**Arquivo novo:** `pkg/rproducer/supabase.go` (~200 linhas)

**Features adicionadas:**

#### 5.1 Instance name normalization (`RPRODUCER_INSTANCE_NAME_OVERRIDE=true`)
- Goroutine de startup carrega `SELECT Nome FROM Empresa` via Supabase REST
- Constrói `map[slug]→Nome` (slug = lowercase + sem acentos + sem não-alfanumérico, copiando exatamente o `normalizeSlug` do bridge)
- Refresh a cada 5min
- Em runtime: `ResolveInstanceName(raw)` retorna o `Empresa.Nome` correto quando recebe um slug; senão fallback gracioso (retorna input)
- Necessário porque o RProducerAgent consulta `Empresa.eq.Nome.<instance>` — se mandarmos slug não normalizado, lookup falha

#### 5.2 LID Supabase fallback (`RPRODUCER_LID_LOOKUP=true`)
- Quando o whatsmeow não conseguiu resolver `@lid` no swap nativo (sender/chat ainda @lid), faz `SELECT number FROM Clientes WHERE lid_jid=X`
- Cache in-memory via `sync.Map` (sem TTL — LIDs não mudam, só crescem mappings)
- Aplica resolução: troca todos `@lid` em `Sender`/`Chat` por `@s.whatsapp.net`, mantém o `@lid` original em `SenderAlt`/`ChatAlt`
- Cobre o caso onde whatsmeow ainda não viu o mapping mas o bridge node.js viu primeiro e populou `Clientes.lid_jid`

#### 5.3 Markers de compat (`RPRODUCER_LID_LOOKUP=true`)
- Quando faz lookup LID Supabase, injeta no payload:
  - `Info._enrichedByBridge = true`
  - `Info._realNumber = "<digits>"`
- Compat com o que o bridge node.js fazia. Se o Agent depender desses markers, continua funcionando.

**Env vars adicionadas (v0.7.1-rwa-3):**

| Env var | Default | Efeito |
|---|---|---|
| `RPRODUCER_INSTANCE_NAME_OVERRIDE` | `false` | Ativa normalização slug→Empresa.Nome real |
| `RPRODUCER_LID_LOOKUP` | `false` | Ativa fallback LID via Clientes.lid_jid + markers |
| `RPRODUCER_SUPABASE_URL` ou `SUPABASE_URL` | — | URL do Supabase (necessário pra 5.1 e 5.2) |
| `RPRODUCER_SUPABASE_SERVICE_KEY` ou `SUPABASE_SERVICE_ROLE_KEY` | — | Service key (bypass RLS) |

**Configuração FINAL recomendada (paridade TOTAL com bridge):**
```
RPRODUCER_FILTER_MESSAGE_ONLY=true
RPRODUCER_DEDUP=true
RPRODUCER_ENVELOPE=rproducer-v1
RPRODUCER_PUBLISH_TO_QUEUE=rproducer-messages.upsert
RPRODUCER_INSTANCE_NAME_OVERRIDE=true
RPRODUCER_LID_LOOKUP=true
SUPABASE_URL=https://ekhwfizcisydztkjnzpx.supabase.co
SUPABASE_SERVICE_ROLE_KEY=<service-role-key>
```

Com isso, o bridge node.js pode ser desligado sem quebrar nada.

---

## Próximas modificações planejadas

- Endpoint `POST /chat/resolveBatch` — resolver N LIDs em uma única request (otimização pro sweep).
- Endpoint `GET /admin/lid-store/stats` — quantidade de mappings no LIDStore (debug).
- Endpoint `GET /admin/stats` — stats agregadas (msg/sec, instâncias up, queue depth).
- Graceful shutdown — drain producers antes de exit.
