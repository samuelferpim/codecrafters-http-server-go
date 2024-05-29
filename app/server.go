package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	StatusOK                  = "200 OK"
	StatusCreated             = "201 Created"
	StatusNotFound            = "404 Not Found"
	StatusInternalServerError = "500 Internal Server Error"
	contentTypePlainText      = "text/plain"
	contentTypeOctetStream    = "application/octet-stream"
	bufferSize                = 4096
)

var (
	contentEncodingGzip = "gzip"
	validPaths          = map[string]struct{}{
		"echo":       {},
		"user-agent": {},
		"files":      {},
	}
)

func HttpResponse(conn net.Conn, status string, body *[]byte, contentType string, contentEncoding *string) {
	statusLine := "HTTP/1.1 " + status + "\r\n"
	headers := "Content-Type: " + contentType + "\r\n"
	if contentEncoding != nil {
		headers += "Content-Encoding: " + *contentEncoding + "\r\n"
	}
	if body != nil {
		bodyLength := strconv.Itoa(len(*body))
		headers += "Content-Length: " + bodyLength + "\r\n\r\n"
		conn.Write([]byte(statusLine + headers))
		conn.Write(*body)
	} else {
		headers += "\r\n"
		conn.Write([]byte(statusLine + headers))
	}
}

func GetPathSegments(request *http.Request) []string {
	return strings.FieldsFunc(request.URL.Path, func(r rune) bool {
		return r == '/'
	})
}

func ProcessPath(request *http.Request) ([]string, error) {
	segments := GetPathSegments(request)
	if len(segments) == 0 {
		return []string{}, nil
	}
	if _, exists := validPaths[segments[0]]; exists {
		return segments[:2], nil
	}
	return nil, errors.New("invalid path")
}

func Handler(conn net.Conn, directory string) {
	defer conn.Close()

	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		logError("Error reading request:", err)
		HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText, nil)
		return
	}

	pathSegments, err := ProcessPath(request)
	if err != nil {
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText, nil)
		return
	}

	if len(pathSegments) == 0 {
		HttpResponse(conn, StatusOK, nil, contentTypePlainText, nil)
		return
	}

	var contentEncoding *string
	acceptEncoding := strings.ToLower(request.Header.Get("Accept-Encoding"))
	if strings.Contains(acceptEncoding, "gzip") {
		contentEncoding = &contentEncodingGzip
	}

	switch pathSegments[0] {
	case "echo":
		handleEcho(conn, pathSegments, contentEncoding)
	case "user-agent":
		handleUserAgent(conn, request, contentEncoding)
	case "files":
		handleFiles(conn, request, directory, pathSegments, contentEncoding)
	default:
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText, nil)
	}
}

func handleEcho(conn net.Conn, pathSegments []string, contentEncoding *string) {
	var responseBody []byte
	if len(pathSegments) > 1 {
		responseBody = []byte(pathSegments[1])
	}
	HttpResponse(conn, StatusOK, &responseBody, contentTypePlainText, contentEncoding)
}

func handleUserAgent(conn net.Conn, request *http.Request, contentEncoding *string) {
	userAgentData := request.UserAgent()
	responseBody := []byte(userAgentData)
	HttpResponse(conn, StatusOK, &responseBody, contentTypePlainText, contentEncoding)
}

func handleFiles(conn net.Conn, request *http.Request, directory string, pathSegments []string, contentEncoding *string) {
	if len(pathSegments) < 2 {
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText, nil)
		return
	}
	filePath := filepath.Join(directory, pathSegments[1])

	switch request.Method {
	case http.MethodGet:
		readFile(conn, filePath, contentEncoding)
	case http.MethodPost:
		writeFile(conn, filePath, request.Body)
	default:
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText, nil)
	}
}

func readFile(conn net.Conn, filePath string, contentEncoding *string) {
	file, err := os.Open(filePath)
	if err != nil {
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText, nil)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText, nil)
		return
	}

	headers := "HTTP/1.1 " + StatusOK + "\r\n" +
		"Content-Type: " + contentTypeOctetStream + "\r\n" +
		"Content-Length: " + strconv.FormatInt(fileInfo.Size(), 10) + "\r\n"
	if contentEncoding != nil {
		headers += "Content-Encoding: " + *contentEncoding + "\r\n"
	}
	headers += "\r\n"
	conn.Write([]byte(headers))

	buffer := make([]byte, bufferSize)
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText, nil)
			return
		}
		if n == 0 {
			break
		}
		conn.Write(buffer[:n])
	}
}

func writeFile(conn net.Conn, filePath string, body io.ReadCloser) {
	fileContents, err := io.ReadAll(body)
	if err != nil {
		HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText, nil)
		return
	}
	err = os.WriteFile(filePath, fileContents, 0644)
	if err != nil {
		HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText, nil)
		return
	}
	HttpResponse(conn, StatusCreated, nil, contentTypePlainText, nil)
}

func logError(message string, err error) {
	fmt.Println(message, err)
}

func main() {
	directory := flag.String("directory", ".", "The directory to serve files from")
	flag.Parse()

	listener, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		logError("Failed to bind to port 4221:", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 4221")

	for {
		conn, err := listener.Accept()
		if err != nil {
			logError("Error accepting connection:", err)
			continue
		}
		go Handler(conn, *directory)
	}
}
