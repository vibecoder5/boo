package drive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"boo/internal/store"
)

func (s *Service) Sync(ctx context.Context, st *store.Store) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := st.Persist(); err != nil {
		return Result{}, fmt.Errorf("не удалось сохранить настройки перед синхронизацией")
	}
	backend, err := s.getBackend(ctx)
	if err != nil {
		s.rememberError(err.Error())
		return Result{}, err
	}
	local, err := st.SyncFiles()
	if err != nil {
		s.rememberError(err.Error())
		return Result{}, fmt.Errorf("не удалось собрать файлы для синхронизации")
	}
	rootID, err := backend.EnsureDir(ctx, "root", rootName)
	if err != nil {
		s.rememberError(err.Error())
		return Result{}, err
	}
	remote, dirs, err := s.listTree(ctx, backend, rootID)
	if err != nil {
		s.rememberError(err.Error())
		return Result{}, err
	}
	localByRel := map[string]store.SyncFile{}
	for _, f := range local {
		localByRel[f.Rel] = f
	}
	var res Result
	downloadedState := false
	for rel, rf := range remote {
		if rf.Folder {
			continue
		}
		safe, ok := store.SafeSyncRel(rel)
		if !ok {
			continue
		}
		lf, hasLocal := localByRel[safe]
		switch {
		case !hasLocal:
			if err := s.downloadOne(ctx, backend, rf, filepath.Join(s.dir, filepath.FromSlash(safe))); err != nil {
				s.rememberError(err.Error())
				return res, err
			}
			res.Downloaded++
			if safe == "state.json" {
				downloadedState = true
			}
		default:
			action, err := compareFiles(lf, rf)
			if err != nil {
				s.rememberError(err.Error())
				return res, err
			}
			switch action {
			case syncSkip:
				res.Skipped++
			case syncDownload:
				if err := s.downloadOne(ctx, backend, rf, lf.Abs); err != nil {
					s.rememberError(err.Error())
					return res, err
				}
				res.Downloaded++
				if safe == "state.json" {
					downloadedState = true
				}
			case syncUpload:
				parent, err := s.parentFor(ctx, backend, dirs, rootID, safe)
				if err != nil {
					s.rememberError(err.Error())
					return res, err
				}
				if err := s.uploadExisting(ctx, backend, parent, lf, rf.ID); err != nil {
					s.rememberError(err.Error())
					return res, err
				}
				res.Uploaded++
			}
		}
		delete(localByRel, safe)
	}
	for _, lf := range localByRel {
		parent, err := s.parentFor(ctx, backend, dirs, rootID, lf.Rel)
		if err != nil {
			s.rememberError(err.Error())
			return res, err
		}
		if err := s.uploadNew(ctx, backend, parent, lf); err != nil {
			s.rememberError(err.Error())
			return res, err
		}
		res.Uploaded++
	}
	if downloadedState {
		if err := st.Reload(); err != nil {
			s.rememberError(err.Error())
			return res, err
		}
	}
	if email, err := backend.About(ctx); err == nil {
		res.Email = email
	} else {
		res.Email = s.loadPersisted().Email
	}
	s.savePersisted(persistedStatus{
		Email:      res.Email,
		LastSync:   time.Now(),
		LastError:  "",
		Uploaded:   res.Uploaded,
		Downloaded: res.Downloaded,
		Skipped:    res.Skipped,
	})
	return res, nil
}

type syncAction int

const (
	syncSkip syncAction = iota
	syncDownload
	syncUpload
)

func compareFiles(local store.SyncFile, remote remoteFile) (syncAction, error) {
	if local.Size == remote.Size && timeClose(local.ModTime, remote.ModTime) {
		return syncSkip, nil
	}
	if remote.MD5 != "" {
		sum, err := fileMD5(local.Abs)
		if err != nil {
			return syncSkip, fmt.Errorf("не удалось прочитать %s", local.Rel)
		}
		if strings.EqualFold(sum, remote.MD5) {
			return syncSkip, nil
		}
	}
	if remote.ModTime.After(local.ModTime.Add(timeSkew / 2)) {
		return syncDownload, nil
	}
	return syncUpload, nil
}

func (s *Service) listTree(ctx context.Context, backend Backend, rootID string) (map[string]remoteFile, map[string]string, error) {
	dirs := map[string]string{"": rootID}
	files := map[string]remoteFile{}
	rootFiles, err := backend.List(ctx, rootID)
	if err != nil {
		return nil, nil, err
	}
	wanted := map[string]bool{"library": true, "covers": true, "dictionaries": true, "ui": true}
	for _, f := range rootFiles {
		if f.Folder {
			if wanted[f.Name] {
				dirs[f.Name] = f.ID
			}
			continue
		}
		if rel, ok := store.SafeSyncRel(f.Name); ok {
			files[rel] = f
		}
	}
	for _, name := range []string{"library", "covers", "dictionaries", "ui"} {
		id, ok := dirs[name]
		if !ok {
			id, err = backend.EnsureDir(ctx, rootID, name)
			if err != nil {
				return nil, nil, err
			}
			dirs[name] = id
		}
		children, err := backend.List(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range children {
			if f.Folder {
				continue
			}
			rel := name + "/" + f.Name
			if safe, ok := store.SafeSyncRel(rel); ok {
				files[safe] = f
			}
		}
	}
	return files, dirs, nil
}

func (s *Service) parentFor(ctx context.Context, backend Backend, dirs map[string]string, rootID, rel string) (string, error) {
	if rel == "state.json" {
		return rootID, nil
	}
	slash := strings.IndexByte(rel, '/')
	if slash <= 0 {
		return "", fmt.Errorf("непонятный путь файла")
	}
	sub := rel[:slash]
	if id, ok := dirs[sub]; ok {
		return id, nil
	}
	id, err := backend.EnsureDir(ctx, rootID, sub)
	if err != nil {
		return "", err
	}
	dirs[sub] = id
	return id, nil
}

func (s *Service) downloadOne(ctx context.Context, backend Backend, rf remoteFile, dest string) error {
	body, err := backend.Download(ctx, rf.ID)
	if err != nil {
		return err
	}
	defer body.Close()
	return writeDownloaded(dest, body, rf.ModTime)
}

func (s *Service) uploadNew(ctx context.Context, backend Backend, parent string, lf store.SyncFile) error {
	f, err := os.Open(lf.Abs)
	if err != nil {
		return fmt.Errorf("не удалось прочитать %s", lf.Rel)
	}
	defer f.Close()
	_, err = backend.Upload(ctx, parent, filepath.Base(lf.Abs), lf.ModTime, f, lf.Size)
	return err
}

func (s *Service) uploadExisting(ctx context.Context, backend Backend, parent string, lf store.SyncFile, id string) error {
	f, err := os.Open(lf.Abs)
	if err != nil {
		return fmt.Errorf("не удалось прочитать %s", lf.Rel)
	}
	defer f.Close()
	if id == "" {
		_, err = backend.Upload(ctx, parent, filepath.Base(lf.Abs), lf.ModTime, f, lf.Size)
		return err
	}
	_, err = backend.Update(ctx, id, lf.ModTime, f, lf.Size)
	return err
}

func writeDownloaded(dest string, r io.Reader, mod time.Time) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("не удалось сохранить файл с Диска")
	}
	tmp := dest + ".drive-tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("не удалось сохранить файл с Диска")
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("не удалось сохранить файл с Диска")
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(dest)
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("не удалось сохранить файл с Диска")
		}
	}
	if !mod.IsZero() {
		_ = os.Chtimes(dest, mod, mod)
	}
	return nil
}

func fileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func timeClose(a, b time.Time) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d < timeSkew
}
