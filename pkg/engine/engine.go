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
	downloadDir string
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
		downloadDir:  downloadDir,
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

func (e *Engine) Torrents() []*torrent.Torrent {
	return e.client.Torrents()
}

func (e *Engine) Find(hashStr string) (*torrent.Torrent, error) {
	var ih metainfo.Hash

	err := ih.FromHexString(hashStr)
	if err != nil {
		return nil, fmt.Errorf("hash invalida: %w", err)
	}

	t, ok := e.client.Torrent(ih)
	if !ok {
		return nil, fmt.Errorf("torrent not found: %s", hashStr)
	}

	return t, nil
}

func (e *Engine) Remove(hashStr string, deleteFiles bool) error {
	t, err := e.Find(hashStr)
	if err != nil {
		return fmt.Errorf("torrent not found :%s", hashStr)
	}

	t.Drop()
	if deleteFiles && t.Name() != "" {
		os.RemoveAll(filepath.Join(e.downloadDir, t.Name()))
	}
	return nil
}
