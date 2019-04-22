package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gocolly/colly"
)

// M is an alias for map.
type M map[string]interface{}

const (
	// EtherscanPendingTx returns table with pending transactions.
	EtherscanPendingTx = "https://etherscan.io/txsPending"
	// InfuraMainNet refers to Infura's main net.
	InfuraMainNet = "https://mainnet.infura.io/v3/9ce23ef47beb48d99c27eda019aed08c"
)

func main() {

	client, err := ethclient.Dial(InfuraMainNet)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	c := colly.NewCollector(
		colly.AllowURLRevisit(),
	)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		from := r.URL.Query().Get("from")
		txPending, err := url.Parse(EtherscanPendingTx)
		if err != nil {
			sendJSON(w, M{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		if from != "" {
			q := txPending.Query()
			q.Set("a", from)
			txPending.RawQuery = q.Encode()
		}
		hashes := make([]string, 0)
		c.OnHTML("#transfers table tbody tr", func(e *colly.HTMLElement) {
			hash := strings.TrimSpace(e.ChildText("td:first-child"))
			if validHash(hash) {
				hashes = append(hashes, hash)
			}
		})
		err = c.Visit(txPending.String())
		if err != nil {
			sendJSON(w, M{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		txs := make([]*types.Transaction, 0)
		for _, hash := range hashes {
			tx, pending, err := client.TransactionByHash(r.Context(), common.HexToHash(hash))
			if err != nil {
				sendJSON(w, M{
					"success": false,
					"error":   err.Error(),
				})
				return
			}
			if pending {
				txs = append(txs, tx)
			}
		}
		sendJSON(w, M{
			"success":      true,
			"transactions": txs,
		})
	})

	srv := &http.Server{
		Addr:         ":" + os.Getenv("PORT"),
		Handler:      mux,
		ReadTimeout:  25 * time.Second,
		WriteTimeout: 25 * time.Second,
	}

	log.Printf("start listening on %s\n", srv.Addr)
	log.Fatalf("%v\n", srv.ListenAndServe())
}

func sendJSON(w http.ResponseWriter, v M) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(v)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(buf.Bytes())
	return err
}

func validHash(s string) bool {
	if s == "" {
		return false
	}
	if !strings.HasPrefix(s, "0x") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return false
	}
	return true
}
