package watch

import "io/fs"

type stamp struct {
	modTime int64
	size    int64
}

type Poller struct {
	fsys  fs.FS
	names []string
	last  map[string]stamp
}

func NewPoller(fsys fs.FS, names []string) *Poller {
	p := &Poller{fsys: fsys, names: names}
	p.last = p.snapshot()
	return p
}

func (p *Poller) Poll() []string {
	now := p.snapshot()
	var changed []string
	for _, name := range p.names {
		if now[name] != p.last[name] {
			changed = append(changed, name)
		}
	}
	p.last = now
	return changed
}

func (p *Poller) snapshot() map[string]stamp {
	stamps := make(map[string]stamp, len(p.names))
	for _, name := range p.names {
		if info, err := fs.Stat(p.fsys, name); err == nil {
			stamps[name] = stamp{info.ModTime().UnixNano(), info.Size()}
		}
	}
	return stamps
}
