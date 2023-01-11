package main

import (
	"fmt"
	"net/url"
	"os/user"
	"strings"
)

type Proxy struct {
	Host     string
	Port     string
	Username string
	Password string
	Scheme   string
	URL      *url.URL
	Path     string
}

// CreateProxy creates a new Proxy by string
func CreateProxy(proxy string) (*Proxy, error) {
	u, err := url.Parse(proxy)

	if err != nil {
		return nil, err
	}

	password, _ := u.User.Password()
	p := &Proxy{
		Host:     u.Hostname(),
		Port:     u.Port(),
		Username: u.User.Username(),
		Password: password,
		Scheme:   strings.ToLower(u.Scheme),
		URL:      u,
	}

	if p.Scheme == "ssh" && p.Username == "" {
		user, err := user.Current()
		if err != nil {
			log.Fatalf(err.Error())
		}
		p.Username = user.Username
	}

	return p, nil
}

func (p *Proxy) Addr() string {
	return fmt.Sprintf("%s:%v", p.Host, p.Port)
}
