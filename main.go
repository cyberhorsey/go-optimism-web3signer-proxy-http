package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type SignRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Address string `json:"address"`
		Data    string `json:"data"`
		Input   string `json:"input"`
	} `json:"params"`
	ID int `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func main() {
	web3SignerURL := os.Getenv("WEB3SIGNER_URL")
	if web3SignerURL == "" {
		panic("WEB3SIGNER_URL environment variable is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		slog.Info("PORT environment variable not set, defaulting to 9000")
		port = "9000"
	}

	http.HandleFunc("/sign", func(w http.ResponseWriter, r *http.Request) {
		var req SignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// the sequencer of optimism op-node p2p lib only uses the input field.
		// the web3signer only has a data field.
		if req.Params.Input == "" {
			http.Error(w, "missing input field", http.StatusBadRequest)
			return
		}
		// so here, we send the input as the data to web3signer.
		payload := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "eth_sign",
			"params":  []interface{}{req.Params.Address, req.Params.Input},
			"id":      req.ID,
		}

		payloadBytes, _ := json.Marshal(payload)
		resp, err := http.Post(web3SignerURL, "application/json", bytes.NewReader(payloadBytes))
		if err != nil {
			http.Error(w, "failed to contact web3signer", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	// here we wrap the optimism op-node interface expectation of a "/healthz" endpoint
	// and make it call the "/upcheck" endpoint of web3signer, proxying the request.
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(web3SignerURL + "/upcheck")
		if err != nil || resp.StatusCode != 200 {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	slog.Info("starting web3signer proxy", "port", port, "web3signerURL", web3SignerURL)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}
