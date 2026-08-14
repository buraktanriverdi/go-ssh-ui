package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"go-ssh/config"
	"go-ssh-ui/internal/scpfs"
	"go-ssh-ui/internal/sshengine"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// FileTransferProgressEvent reports bytes moved so far for an in-flight
// download/upload/remote-copy - the scp protocol reports the total size up
// front (see scpfs.ProgressFunc), so this is a real percentage, not a spinner.
type FileTransferProgressEvent struct {
	TransferID string `json:"transferId"`
	Done       int64  `json:"done"`
	Total      int64  `json:"total"`
}

// FileTransferDoneEvent fires once, when a transfer started by
// Download/Upload/CopyRemoteToRemote finishes (successfully or not).
type FileTransferDoneEvent struct {
	TransferID string `json:"transferId"`
	Error      string `json:"error,omitempty"`
	LocalPath  string `json:"localPath,omitempty"` // set for downloads
}

func init() {
	application.RegisterEvent[FileTransferProgressEvent]("file:progress")
	application.RegisterEvent[FileTransferDoneEvent]("file:done")
}

// FileService is the Wails-bound entry point for the WinSCP-like file
// manager. It's a thin adapter over scpfs.Service (see PLAN.md §5 for why
// that's scp/ssh-based rather than SFTP) - host resolution and the
// async-transfer/event plumbing live here, the actual protocol and
// transport logic lives in internal/scpfs and internal/sshengine.
type FileService struct {
	hosts *HostService
	files *scpfs.Service
}

func NewFileService(hosts *HostService, mgr *sshengine.Manager) *FileService {
	return &FileService{hosts: hosts, files: scpfs.NewService(mgr)}
}

// FileHostRequest addresses a host the same way TerminalService.Connect
// does: by category path + name, which works even for hosts without a
// stable ID.
type FileHostRequest struct {
	CategoryPath []string `json:"categoryPath"`
	Name         string   `json:"name"`
}

func (f *FileService) resolveHost(req FileHostRequest) (config.Host, error) {
	host, ok := f.hosts.FindHostByPath(req.CategoryPath, req.Name)
	if !ok {
		return config.Host{}, fmt.Errorf("host bulunamadı: %s", req.Name)
	}
	return host, nil
}

// CanBrowse reports whether host supports the file manager at all - see
// sshengine.CanRunOneOff: hosts using multi-step Commands (SEND/SENDPASS/
// dzdo escalation) aren't supported in v1 since a one-off non-interactive
// command would skip that scripting entirely.
func (f *FileService) CanBrowse(req FileHostRequest) (bool, error) {
	host, err := f.resolveHost(req)
	if err != nil {
		return false, err
	}
	return sshengine.CanRunOneOff(host), nil
}

type ListRequest struct {
	FileHostRequest
	Dir string `json:"dir"`
}

func (f *FileService) List(req ListRequest) ([]scpfs.Entry, error) {
	host, err := f.resolveHost(req.FileHostRequest)
	if err != nil {
		return nil, err
	}
	return f.files.List(host, req.Dir)
}

// ListLocal lists a local directory for the file manager's local pane (no
// SSH involved) - dir defaults to the user's home directory if empty.
func (f *FileService) ListLocal(dir string) ([]scpfs.Entry, error) {
	if strings.TrimSpace(dir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = home
	}
	return scpfs.ListLocal(dir)
}

// HomeDir reports the local user's home directory, used to initialise the
// file manager's local pane.
func (f *FileService) HomeDir() (string, error) {
	return os.UserHomeDir()
}

type MkdirRequest struct {
	FileHostRequest
	Dir string `json:"dir"`
}

func (f *FileService) Mkdir(req MkdirRequest) error {
	host, err := f.resolveHost(req.FileHostRequest)
	if err != nil {
		return err
	}
	return f.files.Mkdir(host, req.Dir)
}

type DeleteRequest struct {
	FileHostRequest
	Target string `json:"target"`
}

func (f *FileService) Delete(req DeleteRequest) error {
	host, err := f.resolveHost(req.FileHostRequest)
	if err != nil {
		return err
	}
	return f.files.Delete(host, req.Target)
}

type RenameRequest struct {
	FileHostRequest
	From string `json:"from"`
	To   string `json:"to"`
}

func (f *FileService) Rename(req RenameRequest) error {
	host, err := f.resolveHost(req.FileHostRequest)
	if err != nil {
		return err
	}
	return f.files.Rename(host, req.From, req.To)
}

func (f *FileService) emitProgress(transferID string, done, total int64) {
	application.Get().Event.Emit("file:progress", FileTransferProgressEvent{TransferID: transferID, Done: done, Total: total})
}

func (f *FileService) emitDone(transferID string, err error, localPath string) {
	ev := FileTransferDoneEvent{TransferID: transferID, LocalPath: localPath}
	if err != nil {
		ev.Error = err.Error()
	}
	application.Get().Event.Emit("file:done", ev)
}

type DownloadRequest struct {
	FileHostRequest
	RemotePath string `json:"remotePath"`
}

type TransferResponse struct {
	TransferID string `json:"transferId"`
}

// Download starts streaming remotePath to ~/Downloads (de-duplicating the
// filename if one already exists there) and returns a transfer ID
// immediately; progress/completion follow as "file:progress"/"file:done"
// events on that ID - the same immediate-ID-then-events shape as
// TerminalService.Connect, for the same reason (a multi-second operation
// that a blocking RPC call isn't a good fit for).
func (f *FileService) Download(req DownloadRequest) (TransferResponse, error) {
	host, err := f.resolveHost(req.FileHostRequest)
	if err != nil {
		return TransferResponse{}, err
	}

	localPath, err := uniqueDownloadPath(path.Base(req.RemotePath))
	if err != nil {
		return TransferResponse{}, err
	}

	transferID := uuid.NewString()
	go func() {
		out, err := os.Create(localPath)
		if err != nil {
			f.emitDone(transferID, fmt.Errorf("yerel dosya oluşturulamadı: %w", err), "")
			return
		}
		_, err = f.files.Download(host, req.RemotePath, out, func(done, total int64) {
			f.emitProgress(transferID, done, total)
		})
		_ = out.Close()
		if err != nil {
			_ = os.Remove(localPath)
			f.emitDone(transferID, err, "")
			return
		}
		f.emitDone(transferID, nil, localPath)
	}()
	return TransferResponse{TransferID: transferID}, nil
}

type UploadRequest struct {
	FileHostRequest
	RemoteDir string `json:"remoteDir"`
	LocalPath string `json:"localPath"`
}

// Upload starts streaming a local file (picked via PickLocalFile or dropped
// from Finder) to remoteDir, returning a transfer ID immediately - see
// Download's doc comment for the event-based progress/completion shape.
func (f *FileService) Upload(req UploadRequest) (TransferResponse, error) {
	host, err := f.resolveHost(req.FileHostRequest)
	if err != nil {
		return TransferResponse{}, err
	}

	stat, err := os.Stat(req.LocalPath)
	if err != nil {
		return TransferResponse{}, fmt.Errorf("yerel dosya okunamadı: %w", err)
	}
	if stat.IsDir() {
		return TransferResponse{}, fmt.Errorf("klasör yüklemesi henüz desteklenmiyor")
	}

	transferID := uuid.NewString()
	go func() {
		in, err := os.Open(req.LocalPath)
		if err != nil {
			f.emitDone(transferID, fmt.Errorf("yerel dosya açılamadı: %w", err), "")
			return
		}
		defer in.Close()

		mode := fmt.Sprintf("%04o", stat.Mode().Perm())
		err = f.files.Upload(host, req.RemoteDir, filepath.Base(req.LocalPath), in, stat.Size(), mode, func(done, total int64) {
			f.emitProgress(transferID, done, total)
		})
		f.emitDone(transferID, err, "")
	}()
	return TransferResponse{TransferID: transferID}, nil
}

// PickLocalFile opens a native "choose file" dialog for uploads, returning
// the chosen path (or "" if cancelled).
func (f *FileService) PickLocalFile() (string, error) {
	selected, err := application.Get().Dialog.OpenFile().SetTitle("Yüklenecek dosyayı seç").PromptForSingleSelection()
	if err != nil {
		return "", nil // user cancelled - not an error condition for the UI
	}
	return selected, nil
}

// RevealInFinder shows a downloaded file in Finder - the fallback for
// Wails not supporting native OS drag-out of a file from the webview (see
// PLAN.md §5).
func (f *FileService) RevealInFinder(localPath string) error {
	return exec.Command("open", "-R", localPath).Run()
}

// uniqueDownloadPath returns ~/Downloads/name, or ~/Downloads/name (2),
// (3), ... if that path already exists - Finder's own convention, so a
// re-download never silently clobbers a previous one.
func uniqueDownloadPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("indirilenler klasörü oluşturulamadı: %w", err)
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	candidate := filepath.Join(dir, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}
