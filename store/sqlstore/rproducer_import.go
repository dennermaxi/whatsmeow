// RProducer fork addition — nao existe no upstream whatsmeow.
// Adiciona metodo pra importar prekeys em lote quando estamos "herdando" uma sessao
// vinda do WhatsApp Web (via browserbridge, pra contornar Shortcake).
//
// Ref: pkg/browserbridge no RProducerWA.
package sqlstore

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/types"
)

// PreKeyImport eh um prekey vindo do WA Web (ja decodificado do IndexedDB).
// Pub eh opcional/nao-usado — Signal salva so a priv (deriva pub sob demanda).
type PreKeyImport struct {
	KeyID uint32
	Priv  []byte // 32 bytes Curve25519 private key
}

const rproducerInsertPreKeyQuery = `INSERT INTO whatsmeow_pre_keys (jid, key_id, key, uploaded) VALUES ($1, $2, $3, $4) ON CONFLICT (jid, key_id) DO NOTHING`

// ImportPreKeysForJID insere um batch de prekeys herdados. Marca todos como uploaded=true
// (a assumption eh que se veio do WA Web, o server ja conhece essas chaves).
//
// Usa uma unica transacao pra insercao em lote.
func (c *Container) ImportPreKeysForJID(ctx context.Context, jid types.JID, preKeys []PreKeyImport) error {
	if len(preKeys) == 0 {
		return nil
	}
	return c.db.DoTxn(ctx, nil, func(ctx context.Context) error {
		for _, pk := range preKeys {
			if len(pk.Priv) != 32 {
				return fmt.Errorf("prekey %d: priv key tem %d bytes, esperado 32", pk.KeyID, len(pk.Priv))
			}
			if _, err := c.db.Exec(ctx, rproducerInsertPreKeyQuery, jid, pk.KeyID, pk.Priv, true); err != nil {
				return fmt.Errorf("insert prekey %d: %w", pk.KeyID, err)
			}
		}
		return nil
	})
}
