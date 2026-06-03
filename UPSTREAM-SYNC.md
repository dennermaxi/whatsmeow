# Como sincronizar com upstream Evolution Go

Quando o upstream lançar uma nova versão (ex: `0.8.0`), este é o passo-a-passo pra trazer pro fork sem perder nossas modificações.

## Setup inicial (uma única vez)

```bash
# Adicionar upstream como remote
git remote add upstream https://github.com/EvolutionAPI/evolution-go.git
git remote -v  # confirma: origin=dennermaxi/RProducerWA, upstream=EvolutionAPI/evolution-go
```

## Rebase quando upstream tagga nova versão

```bash
# 1) Fetch tags do upstream
git fetch upstream --tags

# 2) Conferir o que mudou
git log HEAD..upstream/main --oneline | head -30

# 3) Criar branch de rebase
git checkout -b rebase-vX.Y.Z

# 4) Rebase em cima da tag nova
git rebase upstream/vX.Y.Z

# 5) Resolver conflitos (esperados nos arquivos com [RPRODUCER]):
#    - pkg/events/websocket/websocket_producer.go (safeConn)
#    - pkg/chat/handler/chat_handler.go (ResolveLid handler)
#    - pkg/chat/service/chat_service.go (ResolveLid service)
#    - pkg/routes/routes.go (rota /chat/resolveLid)
#
# Procure por `[RPRODUCER]` em cada arquivo e mantenha nossos trechos —
# o resto deve aceitar a versão upstream (`git checkout --theirs`).

git checkout --theirs <arquivos-sem-[RPRODUCER]>
# Pros arquivos com [RPRODUCER], edição manual mantendo nossos trechos.

# 6) Verificar build local
docker build --platform linux/amd64 -t local/rproducer-wa:rebase-test .

# 7) Verificar comentários [RPRODUCER] ainda presentes
grep -rn "\[RPRODUCER\]" pkg/ | wc -l   # deve bater com count de RPRODUCER-CHANGES.md

# 8) Atualizar VERSION + tag
echo "X.Y.Z" > VERSION
git commit -am "sync: vX.Y.Z from upstream"
git tag vX.Y.Z-rwa-1

# 9) Atualizar RPRODUCER-CHANGES.md com nova base
# Editar o cabeçalho "vX.Y.Z-rwa-N (base: upstream tag X.Y.Z)"

# 10) Push
git checkout main
git merge rebase-vX.Y.Z --ff-only
git push origin main --tags

# CI vai buildar + pushar dennermaxi/rproducer-wa:vX.Y.Z-rwa-1
```

## Atalho: cherry-pick em vez de rebase

Se o upstream pulou várias versões de uma vez ou se o rebase ficar complicado, alternativa é começar do zero:

```bash
git checkout -b clean-vX.Y.Z upstream/vX.Y.Z
# Cherry-pick os commits [RPRODUCER]
git cherry-pick <hash-do-commit-safeConn>
git cherry-pick <hash-do-commit-ResolveLid>
git cherry-pick <hash-do-commit-branding>
```

## Quando NÃO sincronizar

- Upstream tem patches que NÃO são compatíveis com nosso stack (ex: mudou schema do whatsmeow de forma incompatível).
- Upstream introduziu dependência paga ou breaking change na API que afeta nosso CRM.

Em qualquer caso, segurar a sync e abrir thread no repo `RProducerCRM/issues` discutindo se vale.

## Cadência sugerida

Checar upstream **1x por semana** (segunda-feira). Sync quando houver tag nova ou >5 commits relevantes. Não sync por sync — se o que mudou no upstream não afeta nosso uso, esperar acumular.
