package main

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// initProxy configures the global ProxyClient based on PROXY_ADDR.
func initProxy() {
	proxyEnabled = proxyStr != ""

	if !proxyEnabled {
		log.Println("Proxy disabled - using direct connection")
		ProxyClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				DisableKeepAlives:   false,
			},
		}
		return
	}

	parsed, err := url.Parse(proxyStr)
	if err != nil {
		logger.Fatalf("Invalid PROXY_ADDR: %v", err)
	}

	// SOCKS5 proxy
	if parsed.Scheme == "socks5" {
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{
				User:     parsed.User.Username(),
				Password: password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			logger.Fatalf("SOCKS5 proxy setup failed: %v", err)
		}

		transport := &http.Transport{
			Dial:                dialer.Dial,
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   false,
			DisableCompression:  false,
		}

		ProxyClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
		logger.Println("SOCKS5 proxy enabled:", proxyStr)
		return
	}

	// HTTP/HTTPS proxy
	transport := &http.Transport{
		Proxy:               http.ProxyURL(parsed),
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false,
	}

	ProxyClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	logger.Println("HTTP/HTTPS proxy enabled:", proxyStr)
}
