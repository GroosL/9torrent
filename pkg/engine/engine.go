package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

type Engine struct {
	client       *torrent.Client
	downloadDir  string
	downloadRate int64
	uploadRate   int64
	rateMu       sync.RWMutex
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

	e := &Engine{
		client:      client,
		downloadDir: downloadDir,
	}

	go e.trackRates()
	return e, nil
}

func (e *Engine) trackRates() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastRead, lastWrite int64
	for range ticker.C {
		if e.client == nil {
			return
		}
		stats := e.client.ConnStats()
		curRead := stats.BytesReadUsefulData.Int64()
		curWrite := stats.BytesWrittenData.Int64()

		dl := curRead - lastRead
		ul := curWrite - lastWrite
		if dl < 0 {
			dl = 0
		}
		if ul < 0 {
			ul = 0
		}

		lastRead = curRead
		lastWrite = curWrite

		e.rateMu.Lock()
		e.downloadRate = dl
		e.uploadRate = ul
		e.rateMu.Unlock()
	}
}

func (e *Engine) DownloadRate() int64 {
	e.rateMu.RLock()
	defer e.rateMu.RUnlock()
	return e.downloadRate
}

func (e *Engine) UploadRate() int64 {
	e.rateMu.RLock()
	defer e.rateMu.RUnlock()
	return e.uploadRate
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
