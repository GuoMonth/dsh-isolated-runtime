package kubernetes

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const serviceAccountDir = "/var/run/secrets/kubernetes.io/serviceaccount"

type apiError struct {
	Status int
	Body   string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("kubernetes API status %d: %s", e.Status, e.Body)
}

func isStatus(err error, status int) bool {
	apiErr, ok := err.(*apiError)
	return ok && apiErr.Status == status
}

type restClient struct {
	baseURL   string
	token     string
	namespace string
	http      *http.Client
}

func newInClusterRESTClient(namespace string) (*restClient, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	if port == "" {
		port = os.Getenv("KUBERNETES_SERVICE_PORT")
	}
	if host == "" || port == "" {
		return nil, fmt.Errorf("kubernetes: in-cluster service environment is unavailable")
	}
	tokenBytes, err := os.ReadFile(filepath.Join(serviceAccountDir, "token"))
	if err != nil {
		return nil, fmt.Errorf("kubernetes: read service account token: %w", err)
	}
	caBytes, err := os.ReadFile(filepath.Join(serviceAccountDir, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("kubernetes: read service account CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("kubernetes: invalid service account CA")
	}
	return &restClient{
		baseURL:   "https://" + host + ":" + port,
		token:     strings.TrimSpace(string(tokenBytes)),
		namespace: namespace,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{
				RootCAs:    roots,
				MinVersion: tls.VersionTLS12,
			}},
		},
	}, nil
}

func (c *restClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode kubernetes response: %w", err)
		}
	}
	return nil
}

func (c *restClient) runtimeCollectionPath() string {
	return "/apis/runtime.dsh.io/v1alpha1/namespaces/" + url.PathEscape(c.namespace) + "/runtimes"
}

func (c *restClient) runtimePath(name string) string {
	return c.runtimeCollectionPath() + "/" + url.PathEscape(name)
}

func (c *restClient) podCollectionPath() string {
	return "/api/v1/namespaces/" + url.PathEscape(c.namespace) + "/pods"
}

func (c *restClient) podPath(name string) string {
	return c.podCollectionPath() + "/" + url.PathEscape(name)
}

func (c *restClient) networkPolicyCollectionPath() string {
	return "/apis/networking.k8s.io/v1/namespaces/" + url.PathEscape(c.namespace) + "/networkpolicies"
}

func (c *restClient) networkPolicyPath(name string) string {
	return c.networkPolicyCollectionPath() + "/" + url.PathEscape(name)
}
