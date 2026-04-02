package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	serviceName = "NextFolders"
	configName  = "config.yaml"
)

type Config struct {
	Settings struct {
		ServerURL string `yaml:"server_url" json:"server_url"`
		Username  string `yaml:"username" json:"username"`
	} `yaml:"settings" json:"settings"`
	Templates map[string][]string `yaml:"templates" json:"templates"`
}

// XML Structs for WebDAV PROPFIND
type Multistatus struct {
	XMLName   xml.Name   `xml:"multistatus"`
	Responses []Response `xml:"response"`
}

type Response struct {
	XMLName  xml.Name `xml:"response"`
	Href     string   `xml:"href"`
	Propstat Propstat `xml:"propstat"`
}

type Propstat struct {
	XMLName xml.Name `xml:"propstat"`
	Prop    Prop     `xml:"prop"`
}

type Prop struct {
	XMLName      xml.Name     `xml:"prop"`
	ResourceType ResourceType `xml:"resourcetype"`
}

type ResourceType struct {
	XMLName    xml.Name `xml:"resourcetype"`
	Collection *string  `xml:"collection"` // pointer so we know if it was present
}

type App struct {
	ctx        context.Context
	configPath string
}

func NewApp() *App {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	if strings.Contains(dir, "Temp") || strings.Contains(dir, "go-build") {
		dir, _ = os.Getwd()
	}
	return &App{
		configPath: filepath.Join(dir, configName),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		defaultConfig := Config{}
		defaultConfig.Settings.ServerURL = "https://cloud.example.com"
		defaultConfig.Settings.Username = "admin"
		defaultConfig.Templates = map[string][]string{
			"Разработка": {
				"Документация/ТЗ",
				"Документация/Черновики",
				"Исходники/Go",
				"Исходники/React",
			},
			"Дизайн проект": {
				"Референсы",
				"Макеты",
				"Экспорт",
			},
		}
		data, _ := yaml.Marshal(&defaultConfig)
		os.WriteFile(a.configPath, data, 0644)
	}
}

func (a *App) SaveCredentials(username, password string) error {
	return keyring.Set(serviceName, username, password)
}

func (a *App) GetPassword(username string) (string, error) {
	return keyring.Get(serviceName, username)
}

func (a *App) LoadConfig() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return cfg, fmt.Errorf("ошибка чтения файла конфигурации: %v", err)
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("ошибка парсинга конфигурации: %v", err)
	}
	return cfg, nil
}

func (a *App) SaveConfig(serverURL, username string) error {
	cfg, err := a.LoadConfig()
	if err != nil {
		cfg = Config{}
	}
	cfg.Settings.ServerURL = serverURL
	cfg.Settings.Username = username
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, data, 0644)
}

func (a *App) ListFolders(serverURL, username, password, path string) ([]string, error) {
	serverURL = strings.TrimRight(serverURL, "/")
	path = strings.TrimPrefix(path, "/")
	
	encodedPath := strings.ReplaceAll(path, " ", "%20")
	var webdavURL string
	if path == "" {
		webdavURL = fmt.Sprintf("%s/remote.php/webdav/", serverURL)
	} else {
		webdavURL = fmt.Sprintf("%s/remote.php/webdav/%s/", serverURL, encodedPath)
	}

	req, err := http.NewRequest("PROPFIND", webdavURL, nil)
	if err != nil {
		return nil, err
	}
	
	req.SetBasicAuth(username, password)
	req.Header.Set("Depth", "1")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ошибка %d при доступе к серверу", resp.StatusCode)
	}

	var m Multistatus
	decoder := xml.NewDecoder(resp.Body)
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}

	var folders []string
	
	expectedParentPath := "/remote.php/webdav"
	if path != "" {
		expectedParentPath = expectedParentPath + "/" + path
	}
	expectedParentPath = strings.TrimRight(expectedParentPath, "/")

	for _, r := range m.Responses {
		decodedHref, _ := url.PathUnescape(r.Href)
		decodedHrefTrimmed := strings.TrimRight(decodedHref, "/")
		
		if r.Propstat.Prop.ResourceType.Collection != nil {
			if decodedHrefTrimmed == expectedParentPath {
				continue
			}
			parts := strings.Split(decodedHrefTrimmed, "/")
			if len(parts) > 0 {
				folders = append(folders, parts[len(parts)-1])
			}
		}
	}
	return folders, nil
}

func (a *App) CreateStructure(serverURL, username, password, basePath, targetFolder string, templatePaths []string) []string {
	var logs []string

	serverURL = strings.TrimRight(serverURL, "/")
	basePath = strings.TrimLeft(basePath, "/")
	basePath = strings.TrimRight(basePath, "/")
	targetFolder = strings.Trim(targetFolder, "/")
	
	webdavURL := fmt.Sprintf("%s/remote.php/webdav", serverURL)

	logMsg := func(msg string) {
		logs = append(logs, msg)
	}

	client := &http.Client{}

	createDir := func(path string) error {
		encodedPath := strings.ReplaceAll(path, " ", "%20")
		urlPath := fmt.Sprintf("%s/%s", webdavURL, encodedPath)

		req, err := http.NewRequest("MKCOL", urlPath, nil)
		if err != nil {
			return err
		}
		
		req.SetBasicAuth(username, password)
		resp, err := client.Do(req)
		
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated {
			logMsg(fmt.Sprintf("✓ Создана: %s", path))
			return nil
		} else if resp.StatusCode == http.StatusMethodNotAllowed {
			logMsg(fmt.Sprintf("- Пропущена (уже существует): %s", path))
			return nil
		} else {
			return fmt.Errorf("ошибка %d", resp.StatusCode)
		}
	}

	uniquePaths := make(map[string]bool)
	var orderedPaths []string

	buildPath := func(path string) {
		if path != "" && !uniquePaths[path] {
			uniquePaths[path] = true
			orderedPaths = append(orderedPaths, path)
		}
	}

	// 1. Process base path
	currentBuild := ""
	if basePath != "" {
		parts := strings.Split(basePath, "/")
		for _, part := range parts {
			if currentBuild == "" {
				currentBuild = part
			} else {
				currentBuild = currentBuild + "/" + part
			}
			buildPath(currentBuild)
		}
	}

	// 2. Add target folder
	if targetFolder != "" {
		if currentBuild == "" {
			currentBuild = targetFolder
		} else {
			currentBuild = currentBuild + "/" + targetFolder
		}
		buildPath(currentBuild)
	}

	rootPath := currentBuild

	// 3. Process templates
	for _, tPath := range templatePaths {
		tPath = strings.Trim(tPath, "/")
		parts := strings.Split(tPath, "/")
		
		pBuild := rootPath
		for _, part := range parts {
			if pBuild == "" {
				pBuild = part
			} else {
				pBuild = pBuild + "/" + part
			}
			buildPath(pBuild)
		}
	}

	for _, p := range orderedPaths {
		if p == "" {
			continue
		}
		err := createDir(p)
		if err != nil {
			logMsg(fmt.Sprintf("✗ Ошибка при создании %s: %v", p, err))
			break 
		}
	}

	logMsg("✅ Операция завершена.")
	return logs
}
