package vfs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/anacrolix/torrent"

	"torrent-cli/pkg/engine"
)

type RootNode struct {
	fs.Inode
	engine *engine.Engine
}

type StatusNode struct {
	fs.Inode
	engine *engine.Engine
}

type CtlNode struct {
	fs.Inode
  engine *engine.Engine
}

type TorrentsDirNode struct {
  fs.Inode
  engine *engine.Engine
}

type TorrentNode struct {
	fs.Inode
	torrent *torrent.Torrent
	engine *engine.Engine
}

type TorrentNameNode struct {
	fs.Inode
	torrent *torrent.Torrent
}

type TorrentStatusNode struct {
	fs.Inode
	torrent *torrent.Torrent
}

type TorrentCtlNode struct {
	fs.Inode
	torrent *torrent.Torrent
	engine *engine.Engine
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (r *RootNode) OnAdd(ctx context.Context) {
	status := &StatusNode{engine: r.engine}
	r.AddChild("status", r.NewPersistentInode(ctx, status, fs.StableAttr{Mode: fuse.S_IFREG}), false)

	ctl := &CtlNode{engine: r.engine}
	r.AddChild("ctl", r.NewPersistentInode(ctx, ctl, fs.StableAttr{Mode: fuse.S_IFREG}), false)

	torrents := &TorrentsDirNode{engine: r.engine}
	r.AddChild("torrents", r.NewPersistentInode(ctx, torrents, fs.StableAttr{Mode: fuse.S_IFDIR}), false)
}

func (s *StatusNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (s *StatusNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	var downloading, seeding, stalled, paused, finished int

	for _, t := range s.engine.Torrents() {
		stats := t.Stats()
		if (t.Length() > 0 && t.BytesCompleted() >= t.Length()) {
			finished++
			if stats.ActivePeers > 0 {
				seeding++
			}
		} else if stats.ActivePeers > 0 {
			downloading++
		} else {
			stalled++
		}
	}
	
	output := fmt.Sprintf(
	    "Torrents downloading: %d\n"+
	    "Torrents seeding: %d\n"+
	    "Torrents stalled: %d\n"+
	    "Torrents paused: %d\n"+
	    "Torrents finished: %d\n"+
	    "Total download speed: %s\n"+
	    "Total upload speed: %s\n",
	    downloading, seeding, stalled, paused, finished,
	    formatSpeed(s.engine.DownloadRate()),
	    formatSpeed(s.engine.UploadRate()),
	)
	data := []byte(output)
	if off >= int64(len(data)) {
	    return fuse.ReadResultData(nil), 0
	}
	return fuse.ReadResultData(data[off:]), 0
}

func formatSpeed(bytesPerSec int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytesPerSec >= GB:
		return fmt.Sprintf("%.2f GB/s", float64(bytesPerSec)/float64(GB))
	case bytesPerSec >= MB:
		return fmt.Sprintf("%.2f MB/s", float64(bytesPerSec)/float64(MB))
	case bytesPerSec >= KB:
		return fmt.Sprintf("%.2f KB/s", float64(bytesPerSec)/float64(KB))
	default:
		return fmt.Sprintf("%d B/s", bytesPerSec)
	}
}

func (c *CtlNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0200
	return 0
}

func (c *CtlNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	return 0
}

func (c *CtlNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (c *CtlNode) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	cmd := strings.TrimSpace(string(data))

	if strings.HasPrefix(cmd, "add ") {
		target := strings.TrimSpace(cmd[4:])
		var t *torrent.Torrent
		var err error
		if strings.HasPrefix(target, "magnet:") {
			t, err = c.engine.AddMagnet(target)
		} else {
			t, err = c.engine.AddTorrentFile(target)
		}
		if err == nil && t != nil {
			go func() {
				<-t.GotInfo()
				t.DownloadAll()
			}()
		}
	}
	return uint32(len(data)), 0
}

func (td *TorrentsDirNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := []fuse.DirEntry{}
	for _, t := range td.engine.Torrents() {
		ih := t.InfoHash().HexString()
		entries = append(entries, fuse.DirEntry{Name: ih, Mode: fuse.S_IFDIR,})
	}
	return fs.NewListDirStream(entries), 0
}

func (td *TorrentsDirNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall. Errno) {
	t, err := td.engine.Find(name)
	if err != nil {
		return nil, syscall.ENOENT
	}
	node := &TorrentNode{torrent: t, engine: td.engine}
	return td.NewPersistentInode(ctx, node, fs.StableAttr{Mode: fuse.S_IFDIR}), 0
}

func (tn *TorrentNode) OnAdd(ctx context.Context) {
	nameNode := &TorrentNameNode{torrent: tn.torrent}
	tn.AddChild("name", tn.NewPersistentInode(ctx, nameNode, fs.StableAttr{Mode: fuse.S_IFREG}), false)
	
	statusNode := &TorrentStatusNode{torrent: tn.torrent}
	tn.AddChild("status", tn.NewPersistentInode(ctx, statusNode, fs.StableAttr{Mode: fuse.S_IFREG}), false)

	ctlNode := &TorrentCtlNode{torrent: tn.torrent, engine: tn.engine}
	tn.AddChild("ctl", tn.NewPersistentInode(ctx,ctlNode, fs.StableAttr{Mode: fuse.S_IFREG}), false)
}

func (tn *TorrentNameNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (tn *TorrentNameNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	name := tn.torrent.Name() + "\n"
	data := []byte(name)
	if off >= int64(len(data)) {
		return fuse.ReadResultData(nil), 0
	}
	return fuse.ReadResultData(data[off:]), 0
}

func (tn *TorrentStatusNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (tn *TorrentStatusNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	t := tn.torrent

	if t.Info() == nil || t.Length() == 0 {
		data := []byte("waiting metadata...\n")
		if off >= int64(len(data)) {
			return fuse.ReadResultData(nil), 0
		}
		return fuse.ReadResultData(data[off:]), 0
	}

	completed := t.BytesCompleted()
	total := t.Length()
	percent := float64(completed) / float64(total) * 100
	stats := t.Stats()

	var state string
	if completed >= total {
		state = "Completed"
		if stats.ActivePeers > 0 {
			state = "Seeding"
		}
	} else if stats.ActivePeers > 0 {
		state = "Downloading"
	} else {
		state = "Stalled"
	}

	output := fmt.Sprintf(
		"Name: %s\n"+
			"State: %s\n"+
			"Progress: %.2f%%\n"+
			"Downloaded: %s / %s\n"+
			"Active peers: %d (Total: %d)\n",
		t.Name(),
		state,
		percent,
		formatBytes(completed),
		formatBytes(total),
		stats.ActivePeers,
		stats.TotalPeers,
	)

	data := []byte(output)
	if off >= int64(len(data)) {
		return fuse.ReadResultData(nil), 0
	}
	return fuse.ReadResultData(data[off:]), 0
}

func (tn *TorrentCtlNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (tn *TorrentCtlNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = fuse.S_IFREG | 0200
	return 0
}

func (tn *TorrentCtlNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	return 0
}

func (tn *TorrentCtlNode) handlePause() syscall.Errno {
	tn.torrent.DisallowDataDownload()
	if tn.torrent.Info() != nil {
		tn.torrent.CancelPieces(0, tn.torrent.NumPieces())
	}
	return 0
}

func (tn *TorrentCtlNode) handleResume() syscall.Errno {
	tn.torrent.AllowDataDownload()
	tn.torrent.DownloadAll()
	return 0
}

func (tn *TorrentCtlNode) handleDelete() syscall.Errno {
	ih := tn.torrent.InfoHash().HexString()
	if err := tn.engine.Remove(ih, false); err != nil {
		return syscall.EIO
	}
	return 0
}

func (tn *TorrentCtlNode) handlePurge() syscall.Errno {
	ih := tn.torrent.InfoHash().HexString()
	if err := tn.engine.Remove(ih, true); err != nil {
		return syscall.EIO
	}
	return 0
}

func (tn *TorrentCtlNode) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	cmd := strings.TrimSpace(string(data))

	handlers := map[string]func() syscall.Errno {
		"pause": tn.handlePause,
		"resume": tn.handleResume,
		"delete": tn.handleDelete,
		"purge": tn.handlePurge,
	}

	handler, exists := handlers[cmd]
	if !exists {
		return 0, syscall.EINVAL
	}

	if errno := handler(); errno != 0 {
		return 0, errno
	}

	return uint32(len(data)), 0
}

func Mount(mountDir string, eng *engine.Engine) (*fuse.Server, error) {
	root := &RootNode{engine: eng}

	opts := &fs.Options{}
	opts.UID = uint32(os.Getuid())
	opts.GID = uint32(os.Getgid())

	return fs.Mount(mountDir, root, opts)
}
