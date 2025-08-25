package proxy

import (
	"errors"
	"log"
	"net/url"
	"sync"
)

var ErrNoProxiesAvailable = errors.New("no proxy URLs available")
var ErrAllProxiesExhausted = errors.New("all available proxies have been exhausted")

type Manager struct {
	proxies      []*url.URL
	currentIndex int
	mutex        sync.Mutex
}

func NewManager(proxyStrings []string) (*Manager, error) {
	if len(proxyStrings) == 0 || (len(proxyStrings) == 1 && proxyStrings[0] == "") {
		return nil, ErrNoProxiesAvailable
	}

	var proxies []*url.URL
	for _, p := range proxyStrings {
		proxyURL, err := url.Parse(p)
		if err != nil {
			log.Printf("Warning: could not parse proxy URL '%s', skipping. Error: %v", p, err)
			continue
		}
		proxies = append(proxies, proxyURL)
	}

	if len(proxies) == 0 {
		return nil, errors.New("no valid proxy URLs could be parsed")
	}

	return &Manager{
		proxies:      proxies,
		currentIndex: 0,
	}, nil
}

func (pm *Manager) GetCurrentProxy() *url.URL {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	if len(pm.proxies) == 0 {
		return nil
	}
	return pm.proxies[pm.currentIndex]
}

func (pm *Manager) RotateProxy() error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	log.Printf("Proxy %s failed or is exhausted. Rotating to the next proxy.", pm.proxies[pm.currentIndex].String())
	pm.currentIndex++

	if pm.currentIndex >= len(pm.proxies) {
		log.Println("WARNING: All proxies have been tried. Resetting to the first proxy.")
		pm.currentIndex = 0
		return ErrAllProxiesExhausted
	}

	log.Printf("Switched to proxy %s.", pm.proxies[pm.currentIndex].String())
	return nil
}

func (pm *Manager) GetTotalProxies() int {
	return len(pm.proxies)
}