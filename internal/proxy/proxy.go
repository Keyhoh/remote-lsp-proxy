package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Keyhoh/devcontainer.lsp/internal/runtime"
)

// Direction describes the namespace translation direction for a URI.
type Direction int

const (
	LocalToRemote Direction = iota
	RemoteToLocal
)

// Config contains the local/remote workspace mapping and the backend used to
// start the remote language server process.
type Config struct {
	LocalWorkspace  string
	RemoteWorkspace string
	Command         string
	Args            []string
	Env             []string
	Backend         runtime.Runner
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
}

// Proxy is the central bridge between the local LSP client and the remote
// language server process.
type Proxy struct {
	cfg    Config
	mapper *PathMapper
}

// PathMapper keeps a local/remote path mapping for file URI translation.
type PathMapper struct {
	LocalRoot  string
	RemoteRoot string
}

func New(cfg Config) *Proxy {
	p := &Proxy{cfg: cfg}
	p.mapper = &PathMapper{
		LocalRoot:  filepath.Clean(cfg.LocalWorkspace),
		RemoteRoot: filepath.Clean(cfg.RemoteWorkspace),
	}
	return p
}

// Run starts the proxy lifecycle and relays LSP traffic between the local client
// and the remote language server.
func (p *Proxy) Run() error {
	if p.cfg.Backend == nil {
		return fmt.Errorf("remote backend is not configured")
	}
	if p.cfg.LocalWorkspace == "" {
		return fmt.Errorf("local workspace is required")
	}
	if p.cfg.RemoteWorkspace == "" {
		return fmt.Errorf("remote workspace is required")
	}
	if p.cfg.Command == "" {
		return fmt.Errorf("remote command is required")
	}
	if p.cfg.Stdin == nil {
		p.cfg.Stdin = os.Stdin
	}
	if p.cfg.Stdout == nil {
		p.cfg.Stdout = os.Stdout
	}
	if p.cfg.Stderr == nil {
		p.cfg.Stderr = os.Stderr
	}

	proc, err := p.cfg.Backend.Start(p.cfg.Command, p.cfg.Args, p.cfg.LocalWorkspace, p.cfg.Env)
	if err != nil {
		return err
	}
	if proc == nil {
		return fmt.Errorf("backend returned a nil process")
	}
	defer proc.Close()

	localToRemoteErr := make(chan error, 1)
	remoteToLocalErr := make(chan error, 1)
	stderrErr := make(chan error, 1)

	go func() {
		localToRemoteErr <- relayMessages(p.cfg.Stdin, proc.Stdin(), func(raw []byte) ([]byte, error) {
			return p.transformJSONRPC(raw, LocalToRemote)
		})
	}()
	go func() {
		remoteToLocalErr <- relayMessages(proc.Stdout(), p.cfg.Stdout, func(raw []byte) ([]byte, error) {
			return p.transformJSONRPC(raw, RemoteToLocal)
		})
	}()
	go func() {
		stderrErr <- copyStream(proc.Stderr(), p.cfg.Stderr)
	}()

	localClosed := false
	remoteClosed := false

	for !(localClosed && remoteClosed) {
		select {
		case err := <-localToRemoteErr:
			if !isEndOfStream(err) {
				return err
			}
			localClosed = true
			_ = proc.Stdin().Close()
		case err := <-remoteToLocalErr:
			if !isEndOfStream(err) {
				return err
			}
			remoteClosed = true
			if waitErr := proc.Wait(); waitErr != nil {
				return waitErr
			}
		case err := <-stderrErr:
			if !isEndOfStream(err) {
				return err
			}
		}
	}
	return nil
}

func isEndOfStream(err error) bool {
	return err == nil || err == io.EOF
}

func relayMessages(reader io.Reader, writer io.Writer, transform func([]byte) ([]byte, error)) error {
	br := bufio.NewReader(reader)
	for {
		msg, err := readJSONRPCMessage(br)
		if err != nil {
			if err == io.EOF {
				return io.EOF
			}
			return err
		}
		out, err := transform(msg)
		if err != nil {
			return err
		}
		if _, err := writer.Write(EncodeJSONRPCMessage(out)); err != nil {
			return err
		}
	}
}

func readJSONRPCMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if line == "\r\n" {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			raw := strings.TrimSpace(line[len("content-length:"):])
			n, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length header: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func copyStream(src io.Reader, dst io.Writer) error {
	if src == nil || dst == nil {
		return nil
	}
	_, err := io.Copy(dst, src)
	if errors := err; errors != nil && errors != io.EOF {
		return err
	}
	return nil
}

func (p *Proxy) translateURI(raw string, dir Direction) string {
	if raw == "" || !strings.HasPrefix(raw, "file:") || p.mapper == nil {
		return raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return raw
	}

	path := filepath.Clean(u.Path)
	mapped := p.mapPath(path, dir)
	if mapped == "" || mapped == path {
		return raw
	}

	u.Path = mapped
	return u.String()
}

func (p *Proxy) translatePath(raw string, dir Direction) string {
	if raw == "" || p.mapper == nil {
		return raw
	}
	return p.mapPath(raw, dir)
}

func (p *Proxy) mapPath(path string, dir Direction) string {
	if path == "" {
		return path
	}
	if dir == LocalToRemote {
		return mapPathBetweenRoots(path, p.mapper.LocalRoot, p.mapper.RemoteRoot)
	}
	return mapPathBetweenRoots(path, p.mapper.RemoteRoot, p.mapper.LocalRoot)
}

func mapPathBetweenRoots(path, fromRoot, toRoot string) string {
	clean := filepath.Clean(path)
	if fromRoot == "" || toRoot == "" {
		return clean
	}
	if clean == fromRoot {
		return toRoot
	}
	rel, err := filepath.Rel(fromRoot, clean)
	if err != nil {
		return clean
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return clean
	}
	return filepath.Join(toRoot, rel)
}

func (p *Proxy) transformJSONRPC(raw []byte, dir Direction) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.Marshal(p.transformValue(v, dir))
}

func (p *Proxy) transformValue(v any, dir Direction) any {
	switch value := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, child := range value {
			if key == "processId" && dir == LocalToRemote {
				out[key] = nil
				continue
			}
			if key == "changes" {
				changes, ok := child.(map[string]any)
				if !ok {
					out[key] = p.transformValue(child, dir)
					continue
				}
				mapped := make(map[string]any, len(changes))
				for uri, next := range changes {
					mapped[p.translateURI(uri, dir)] = p.transformValue(next, dir)
				}
				out[key] = mapped
				continue
			}
			if str, ok := child.(string); ok {
				out[key] = p.transformStringValue(key, str, dir)
				continue
			}
			out[key] = p.transformValue(child, dir)
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = p.transformValue(item, dir)
		}
		return out
	default:
		return v
	}
}

func (p *Proxy) transformStringValue(key, value string, dir Direction) string {
	switch {
	case key == "rootPath":
		return p.translatePath(value, dir)
	case key == "uri" || key == "target" || key == "oldUri" || key == "newUri" || key == "rootUri" || strings.HasSuffix(key, "Uri") || strings.HasSuffix(key, "URI"):
		return p.translateURI(value, dir)
	default:
		return value
	}
}

func EncodeJSONRPCMessage(payload []byte) []byte {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	return append([]byte(header), payload...)
}

func DecodeJSONRPCMessages(r io.Reader) ([]json.RawMessage, error) {
	var buffer bytes.Buffer
	var messages []json.RawMessage
	readBuf := make([]byte, 4096)
	for {
		count, err := r.Read(readBuf)
		if count > 0 {
			if _, werr := buffer.Write(readBuf[:count]); werr != nil {
				return nil, werr
			}
		}
		for {
			msg, consumed, ok, derr := decodeNextJSONRPCMessage(buffer.Bytes())
			if derr != nil {
				return nil, derr
			}
			if !ok {
				break
			}
			messages = append(messages, msg)
			buffer.Next(consumed)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func decodeNextJSONRPCMessage(data []byte) (json.RawMessage, int, bool, error) {
	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
	if headerEnd < 0 {
		return nil, 0, false, nil
	}
	header := data[:headerEnd]
	bodyStart := headerEnd + 4
	lines := bytes.Split(header, []byte("\r\n"))
	contentLength := -1
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("Content-Length:")) {
			_, err := fmt.Sscanf(string(line), "Content-Length: %d", &contentLength)
			if err != nil {
				return nil, 0, false, fmt.Errorf("invalid Content-Length header: %w", err)
			}
			break
		}
	}
	if contentLength < 0 {
		return nil, 0, false, fmt.Errorf("missing Content-Length header")
	}
	messageEnd := bodyStart + contentLength
	if len(data) < messageEnd {
		return nil, 0, false, nil
	}
	body := data[bodyStart:messageEnd]
	return json.RawMessage(body), messageEnd, true, nil
}
