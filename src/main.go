package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HostRules      []HostRule `yaml:"host_rules"`
	AliasUsesHTTPS bool       `yaml:"alias_uses_https"`
	Settings       AppSetting `yaml:"settings"`
}

type HostRule struct {
	Origin string `yaml:"origin"`
	Alias  string `yaml:"alias"`
}

type AppSetting struct {
	Proxy   string `yaml:"proxy"`
	Address string `yaml:"address"`
}

func loadConfigFromYAML(path string) (Config, error) {
	var config Config
	data, err := os.ReadFile(path)

	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, &config)
	return config, err
}

func main() {
	config, err := loadConfigFromYAML("data/config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleProxyRequest(w, r, config, logger)
	})

	logger.Printf("Server is ready to running at http://%s\n", config.Settings.Address)
	logger.Printf("Proxies sites: ")
	for _, rule := range config.HostRules {
		logger.Println(rule.Alias)
	}
	log.Fatal(http.ListenAndServe(config.Settings.Address, nil))
}

func handleProxyRequest(w http.ResponseWriter, r *http.Request, config Config, logger *log.Logger) {
	logger.Printf("%s %s\n", r.Method, r.URL.String())

	aliasHost := r.Host
	originHost := getOriginHost(aliasHost, config.HostRules)
	if originHost == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	originURL := *r.URL
	originURL.Host = originHost
	originURL.Scheme = scheme(true)

	headers := r.Header.Clone()
	headers.Set("Host", originHost)

	//TODO: replace referer, origin... all headers

	resp, err := forwardRequest(r.Method, originURL.String(), headers, r.Body, config.Settings, logger)
	if err != nil {
		http.Error(w, "Proxy error", http.StatusBadGateway)
		logger.Printf("Proxy error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if err := rewriteResponse(resp, config); err != nil {
		http.Error(w, "Failed to rewrite response", http.StatusInternalServerError)
		return
	}

	for k, v := range resp.Header {
		w.Header().Set(k, strings.Join(v, ","))
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func getOriginHost(alias string, rules []HostRule) string {
	for _, rule := range rules {
		if rule.Alias == alias {
			return rule.Origin
		}
	}
	return ""
}

func scheme(HTTPS bool) string {
	if HTTPS {
		return "https"
	}
	return "http"
}

func forwardRequest(method, urlStr string, headers http.Header, body io.ReadCloser, settings AppSetting, logger *log.Logger) (*http.Response, error) {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		body.Close()
	}

	transport := &http.Transport{}
	if settings.Proxy != "" {
		proxyURL, err := url.Parse(settings.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.Header = headers
			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
			return nil
		},
	}

	req, err := http.NewRequest(method, urlStr, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header = headers

	resp, err := client.Do(req)
	if err == nil {
		return resp, nil
	}
	return nil, err
}

func rewriteResponse(resp *http.Response, config Config) error {
	contentEncoding := resp.Header.Get("Content-Encoding")
	contentType := resp.Header.Get("Content-Type")

	if resp.StatusCode != http.StatusNotModified && isTextContent(contentType) {
		content, err := readBodyWithEncoding(resp.Body, contentEncoding)
		if err != nil {
			return fmt.Errorf("failed to process response body: %w", err)
		}
		content = replaceHostURLs(content, config.HostRules, config.AliasUsesHTTPS)

		resp.Header.Del("Content-Encoding")
		resp.Header.Del("Content-Length")
		resp.Header.Del("Transfer-Encoding")
		resp.Body = io.NopCloser(strings.NewReader(content))
	}

	for k, vals := range resp.Header {
		for i, val := range vals {
			val = replaceHostURLs(val, config.HostRules, config.AliasUsesHTTPS)
			vals[i] = val
		}
		resp.Header[k] = vals
	}

	return nil
}

func isTextContent(contentType string) bool {
	return strings.Contains(contentType, "text") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml")
}

func readBodyWithEncoding(body io.Reader, encoding string) (string, error) {
	var reader io.Reader
	if encoding == "gzip" {
		gzReader, err := gzip.NewReader(body)
		if err != nil {
			return "", fmt.Errorf("gzip reader error: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	} else {
		reader = body
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func replaceHostURLs(content string, rules []HostRule, hostWithHTTPS bool) string {
	scheme := scheme(hostWithHTTPS)
	for _, rule := range rules {
		aliasURL := fmt.Sprintf("%s://%s", scheme, rule.Alias)
		for _, originScheme := range []string{"http", "https"} {
			originURL := fmt.Sprintf("%s://%s", originScheme, rule.Origin)
			content = strings.ReplaceAll(content, originURL, aliasURL)
		}
		content = strings.ReplaceAll(content, rule.Origin, rule.Alias)
	}
	return content
}
