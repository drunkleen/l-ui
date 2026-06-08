package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	agentdb "github.com/drunkleen/l-ui/agent/database"
	"github.com/drunkleen/l-ui/internal/logger"
)

type registerRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Version string `json:"version"`
}

type registerResponse struct {
	Success bool `json:"success"`
	Obj     struct {
		NodeID   int    `json:"nodeId"`
		APIToken string `json:"apiToken"`
	} `json:"obj"`
	Msg string `json:"msg"`
}

func tryRegisterWithHub(hubEndpoint, registrationToken string) (string, int, error) {
	hostname := detectHostname()
	address := detectAddress()

	reqBody := registerRequest{
		Name:    hostname,
		Address: address,
		Port:    agentPort(),
		Version: agentVersion(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("marshal registration request: %w", err)
	}

	hubURL := strings.TrimRight(hubEndpoint, "/")
	if !strings.HasPrefix(hubURL, "http") {
		hubURL = "https://" + hubURL
	}
	registerURL := hubURL + "/panel/api/node-registration/register"

	httpReq, err := http.NewRequest(http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("create registration request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", registrationToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("read registration response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("registration failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var regResp registerResponse
	if err := json.Unmarshal(respBody, &regResp); err != nil {
		return "", 0, fmt.Errorf("parse registration response: %w", err)
	}

	if !regResp.Success {
		return "", 0, fmt.Errorf("registration rejected: %s", regResp.Msg)
	}

	logger.Infof("Successfully registered with hub as node %s (ID %d)", hostname, regResp.Obj.NodeID)
	return regResp.Obj.APIToken, regResp.Obj.NodeID, nil
}

func storeNodeSecret(secret string, nodeID int, hubEndpoint string) {
	db := agentdb.GetDB()
	if db == nil {
		return
	}
	existing := &agentdb.NodeSecret{}
	result := db.First(existing)
	if result.Error == nil {
		existing.Secret = secret
		existing.HubNodeID = fmt.Sprintf("%d", nodeID)
		existing.HubEndpoint = hubEndpoint
		db.Save(existing)
	} else {
		db.Create(&agentdb.NodeSecret{
			Secret:      secret,
			HubNodeID:   fmt.Sprintf("%d", nodeID),
			HubEndpoint: hubEndpoint,
		})
	}
}

func loadStoredSecret() string {
	db := agentdb.GetDB()
	if db == nil {
		return ""
	}
	var secret agentdb.NodeSecret
	if err := db.Last(&secret).Error; err != nil {
		return ""
	}
	return secret.Secret
}

func detectHostname() string {
	hostname := strings.TrimSpace(os.Getenv("LUI_NODE_NAME"))
	if hostname != "" {
		return hostname
	}
	h, err := os.Hostname()
	if err == nil && h != "" {
		return h
	}
	return "agent-" + fmt.Sprintf("%d", time.Now().Unix())
}

func detectAddress() string {
	addr := strings.TrimSpace(os.Getenv("LUI_NODE_ADDRESS"))
	if addr != "" {
		return addr
	}
	return strings.TrimSpace(os.Getenv("LUI_WEB_ADDRESS"))
}

func agentPort() int {
	v := strings.TrimSpace(os.Getenv("LUI_WEB_PORT"))
	if v == "" {
		return 2054
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 2054
	}
	return n
}

func agentVersion() string {
	return strings.TrimSpace(os.Getenv("LUI_NODE_VERSION"))
}
