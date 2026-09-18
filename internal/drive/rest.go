package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type restBackend struct {
	http   *http.Client
	api    string
	upload string
}

type driveFileJSON struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	MimeType     string   `json:"mimeType,omitempty"`
	MD5Checksum  string   `json:"md5Checksum,omitempty"`
	Size         string   `json:"size,omitempty"`
	ModifiedTime string   `json:"modifiedTime,omitempty"`
	Parents      []string `json:"parents,omitempty"`
}

type driveListJSON struct {
	Files         []driveFileJSON `json:"files"`
	NextPageToken string          `json:"nextPageToken"`
}

type driveAboutJSON struct {
	User struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"user"`
}

type googleErrorJSON struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (b *restBackend) About(ctx context.Context) (string, error) {
	var about driveAboutJSON
	if err := b.getJSON(ctx, "/drive/v3/about?fields=user(emailAddress)", &about); err != nil {
		return "", err
	}
	return strings.TrimSpace(about.User.EmailAddress), nil
}

func (b *restBackend) EnsureDir(ctx context.Context, parentID, name string) (string, error) {
	if parentID == "" {
		parentID = "root"
	}
	q := fmt.Sprintf("name='%s' and mimeType='%s' and trashed=false and '%s' in parents", escapeDriveQuery(name), folderMIME, parentID)
	files, err := b.listQuery(ctx, q)
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return files[0].ID, nil
	}
	meta := driveFileJSON{
		Name:     name,
		MimeType: folderMIME,
		Parents:  []string{parentID},
	}
	var created driveFileJSON
	if err := b.postJSON(ctx, "/drive/v3/files?fields=id,name,mimeType", meta, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", fmt.Errorf("Google Диск не создал папку")
	}
	return created.ID, nil
}

func (b *restBackend) List(ctx context.Context, parentID string) ([]remoteFile, error) {
	if parentID == "" {
		parentID = "root"
	}
	q := fmt.Sprintf("trashed=false and '%s' in parents", parentID)
	raw, err := b.listQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]remoteFile, 0, len(raw))
	for _, f := range raw {
		out = append(out, parseRemote(f))
	}
	return out, nil
}

func (b *restBackend) Upload(ctx context.Context, parentID, name string, mod time.Time, r io.Reader, size int64) (remoteFile, error) {
	if parentID == "" {
		parentID = "root"
	}
	meta := driveFileJSON{
		Name:         name,
		Parents:      []string{parentID},
		ModifiedTime: formatDriveTime(mod),
	}
	return b.resumable(ctx, http.MethodPost, "/upload/drive/v3/files?uploadType=resumable&fields=id,name,md5Checksum,size,modifiedTime,mimeType", meta, r, size)
}

func (b *restBackend) Update(ctx context.Context, id string, mod time.Time, r io.Reader, size int64) (remoteFile, error) {
	meta := driveFileJSON{ModifiedTime: formatDriveTime(mod)}
	path := "/upload/drive/v3/files/" + url.PathEscape(id) + "?uploadType=resumable&fields=id,name,md5Checksum,size,modifiedTime,mimeType"
	return b.resumable(ctx, http.MethodPatch, path, meta, r, size)
}

func (b *restBackend) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.api+"/drive/v3/files/"+url.PathEscape(id)+"?alt=media", nil)
	if err != nil {
		return nil, err
	}
	res, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("нет связи с Google Диском")
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		return nil, readGoogleError(res)
	}
	return res.Body, nil
}

func (b *restBackend) resumable(ctx context.Context, method, path string, meta driveFileJSON, r io.Reader, size int64) (remoteFile, error) {
	raw, err := json.Marshal(meta)
	if err != nil {
		return remoteFile{}, err
	}
	req, err := http.NewRequestWithContext(ctx, method, b.upload+path, bytes.NewReader(raw))
	if err != nil {
		return remoteFile{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	if size >= 0 {
		req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))
	}
	req.Header.Set("X-Upload-Content-Type", "application/octet-stream")
	res, err := b.http.Do(req)
	if err != nil {
		return remoteFile{}, fmt.Errorf("нет связи с Google Диском")
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return remoteFile{}, readGoogleError(res)
	}
	loc := res.Header.Get("Location")
	if loc == "" {
		return remoteFile{}, fmt.Errorf("Google Диск не принял файл")
	}
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, loc, r)
	if err != nil {
		return remoteFile{}, err
	}
	put.Header.Set("Content-Type", "application/octet-stream")
	if size >= 0 {
		put.ContentLength = size
	}
	putRes, err := b.http.Do(put)
	if err != nil {
		return remoteFile{}, fmt.Errorf("нет связи с Google Диском")
	}
	defer putRes.Body.Close()
	if putRes.StatusCode >= 300 {
		return remoteFile{}, readGoogleError(putRes)
	}
	var created driveFileJSON
	if err := json.NewDecoder(putRes.Body).Decode(&created); err != nil {
		return remoteFile{}, fmt.Errorf("Google Диск вернул непонятный ответ")
	}
	return parseRemote(created), nil
}

func (b *restBackend) listQuery(ctx context.Context, q string) ([]driveFileJSON, error) {
	var all []driveFileJSON
	page := ""
	for {
		u := b.api + "/drive/v3/files?spaces=drive&pageSize=1000&fields=nextPageToken,files(id,name,mimeType,md5Checksum,size,modifiedTime)&q=" + url.QueryEscape(q)
		if page != "" {
			u += "&pageToken=" + url.QueryEscape(page)
		}
		var list driveListJSON
		if err := b.getJSON(ctx, strings.TrimPrefix(u, b.api), &list); err != nil {
			return nil, err
		}
		all = append(all, list.Files...)
		if list.NextPageToken == "" {
			break
		}
		page = list.NextPageToken
	}
	return all, nil
}

func (b *restBackend) getJSON(ctx context.Context, path string, dest any) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.api+path, nil)
	if err != nil {
		return err
	}
	res, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("нет связи с Google Диском")
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return readGoogleError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		return fmt.Errorf("Google Диск вернул непонятный ответ")
	}
	return nil
}

func (b *restBackend) postJSON(ctx context.Context, path string, body any, dest any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.api+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	res, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("нет связи с Google Диском")
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return readGoogleError(res)
	}
	if dest == nil {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		return fmt.Errorf("Google Диск вернул непонятный ответ")
	}
	return nil
}

func parseRemote(f driveFileJSON) remoteFile {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	mod, _ := time.Parse(time.RFC3339Nano, f.ModifiedTime)
	if mod.IsZero() {
		mod, _ = time.Parse(time.RFC3339, f.ModifiedTime)
	}
	return remoteFile{
		ID:      f.ID,
		Name:    f.Name,
		MD5:     strings.ToLower(f.MD5Checksum),
		Size:    size,
		ModTime: mod.UTC(),
		Folder:  f.MimeType == folderMIME,
	}
}

func formatDriveTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func escapeDriveQuery(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func readGoogleError(res *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	var ge googleErrorJSON
	if json.Unmarshal(raw, &ge) == nil && ge.Error.Message != "" {
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			return fmt.Errorf("войдите в Google Диск снова")
		}
		return fmt.Errorf("Google Диск: %s", ge.Error.Message)
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return fmt.Errorf("войдите в Google Диск снова")
	}
	return fmt.Errorf("Google Диск ответил ошибкой (%d)", res.StatusCode)
}
