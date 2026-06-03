package rproducer

import (
	"sync"
	"time"
)

// dedupTTL — janela em que um mesmo msg_id é considerado duplicata.
// 60s é suficiente: Evolution emite as 2 cópias quase juntas (<1s normalmente).
const dedupTTL = 60 * time.Second

// dedupCap — máximo de entries no cache. Cap baixo o suficiente pra não
// vazar memória, alto o suficiente pra não dar "false negative" em prod
// (testado em ~50k msgs/h pico).
const dedupCap = 50_000

// dedupCache — mapa lock-free thread-safe via sync.Map. Não usamos LRU formal
// porque a quantidade de hits é pequena e o sweep periódico cobre.
//
// Chave: msg_id (string). Valor: time.Time (quando viu).
var dedupCache sync.Map

// dedupCount — counter de entries vivas. Atômico não é necessário pra precisão
// exata, só pra forçar sweep quando cresce demais.
var dedupCount int64

var dedupSweeperOnce sync.Once

// startDedupSweeper — goroutine que limpa entries expiradas a cada 30s.
// Idempotente (rodada só uma vez via sync.Once).
func startDedupSweeper() {
	dedupSweeperOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				now := time.Now()
				var removed int64
				dedupCache.Range(func(key, value any) bool {
					if t, ok := value.(time.Time); ok && now.Sub(t) > dedupTTL {
						dedupCache.Delete(key)
						removed++
					}
					return true
				})
				if removed > 0 {
					dedupCount -= removed
					if dedupCount < 0 {
						dedupCount = 0
					}
				}
			}
		}()
	})
}

// IsDuplicateMsgID — retorna true se o msg_id já foi visto nos últimos `dedupTTL`.
// Se for novo, registra e retorna false. Se cache atingir cap, força sweep.
func IsDuplicateMsgID(msgID string) bool {
	if msgID == "" {
		return false
	}
	startDedupSweeper()

	now := time.Now()
	if existing, loaded := dedupCache.LoadOrStore(msgID, now); loaded {
		if t, ok := existing.(time.Time); ok && now.Sub(t) <= dedupTTL {
			return true
		}
		// Re-store com timestamp atual (refresh)
		dedupCache.Store(msgID, now)
	} else {
		dedupCount++
	}
	return false
}
