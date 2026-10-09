package engine

import (
	"fmt"
	"os"
	"path/filepath"


	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

type Engine struct {
	client *torrent.Client
}

func New(downloadDir string) (*Engine, error) {
	if downloadDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve user home dir: %w", err)
		}
		downloadDir = filepath.Join(home, "Downloads")
	}

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = downloadDir

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize torrent client: %w", err)
	}

	return &Engine{
		client: client,
	}, nil
}

func (e *Engine) Close() error {
	if e.client != nil {
		e.client.Close()
	}
	return nil
}

func (e *Engine) AddMagnet (uri string) (*torrent.Torrent, error) {
	t, err := e.client.AddMagnet(uri)
	if err != nil {
		return nil, fmt.Errorf("add magnet uri: %w", err)
	}
	return t, nil
}

func (e *Engine) AddTorrentFile (filePath string) (*torrent.Torrent, error) {
	mi, err := metainfo.LoadFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("load metainfo file: %w", err)
	}

	spec := torrent.TorrentSpecFromMetaInfo(mi)
	
	t, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("adding torrent spec: %w", err)
	}

	return t, nil
}
