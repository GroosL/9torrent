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
	    "Total download speed: 0 B/s\n"+
	    "Total upload speed: 0 B/s\n",
	    downloading, seeding, stalled, paused, finished,
	)
	data := []byte(output)
	if off >= int64(len(data)) {
	    return fuse.ReadResultData(nil), 0
	}
	return fuse.ReadResultData(data[off:]), 0
}

func (c *CtlNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0200
	return 0
}

func (c *CtlNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	return 0
}

func (c *CtlNode) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	cmd := strings.TrimSpace(string(data))

	if strings.HasPrefix(cmd, "add ") {
		target := strings.TrimSpace(cmd[4:])
		if strings.HasPrefix(target, "magnet:") {
			c.engine.AddMagnet(target)
		} else {
			c.engine.AddTorrentFile(target)
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

func Mount(mountDir string, eng *engine.Engine) (*fuse.Server, error) {
	root := &RootNode{engine: eng}

	opts := &fs.Options{}
	opts.UID = uint32(os.Getuid())
	opts.GID = uint32(os.Getgid())

	return fs.Mount(mountDir, root, opts)
}
