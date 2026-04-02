import { useState, useEffect } from 'react';
import './App.css';
import { LoadConfig, SaveConfig, SaveCredentials, GetPassword, CreateStructure, ListFolders } from "../wailsjs/go/main/App";

interface TreeNodeProps {
    name: string;
    path: string;
    selectedPath: string;
    onSelect: (path: string) => void;
    serverUrl: string;
    username: string;
    password: string;
    initialExpanded?: boolean;
}

const TreeNode = ({ name, path, selectedPath, onSelect, serverUrl, username, password, initialExpanded = false }: TreeNodeProps) => {
    const [expanded, setExpanded] = useState(initialExpanded);
    const [children, setChildren] = useState<string[]>([]);
    const [loading, setLoading] = useState(false);
    const [loaded, setLoaded] = useState(false);

    const loadChildren = async () => {
        if (!serverUrl || !username || !password) return;
        setLoading(true);
        try {
            let folders = await ListFolders(serverUrl, username, password, path);
            if (folders && folders.length > 0) {
                folders.sort((a: string, b: string) => a.localeCompare(b));
            }
            setChildren(folders || []);
            setLoaded(true);
        } catch (e) {
            console.error("Tree Error:", e);
        }
        setLoading(false);
    };

    useEffect(() => {
        if (initialExpanded) {
            loadChildren();
        }
    }, [initialExpanded]);

    const handleExpandToggle = async (e: React.MouseEvent) => {
        e.stopPropagation();
        if (!expanded && !loaded) {
            await loadChildren();
        }
        setExpanded(!expanded);
    };

    const handleSelect = (e: React.MouseEvent) => {
        e.stopPropagation();
        onSelect(path);
    };

    return (
        <div className="tree-node">
            <div className={`tree-item ${selectedPath === path ? 'selected' : ''}`} onClick={handleSelect}>
                <span className="chevron" onClick={handleExpandToggle}>
                    {loading ? "⟳" : (expanded ? "▼" : "▶")}
                </span>
                <span className="folder-icon">{path === "" ? "☁️" : "📂"}</span>
                {name || "Root"}
            </div>
            {expanded && (
                <div>
                    {children.map(c => (
                        <TreeNode 
                            key={c} 
                            name={c} 
                            path={path === "" ? c : `${path}/${c}`} 
                            selectedPath={selectedPath} 
                            onSelect={onSelect} 
                            serverUrl={serverUrl} 
                            username={username} 
                            password={password} 
                        />
                    ))}
                    {loaded && children.length === 0 && <div className="tree-node" style={{color: '#64748b', fontSize: '0.8rem', marginLeft: '24px'}}>Пусто</div>}
                </div>
            )}
        </div>
    )
}

function App() {
    const [serverUrl, setServerUrl] = useState("");
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    
    // config state
    const [templates, setTemplates] = useState<Record<string, string[]>>({});
    const [selectedTemplate, setSelectedTemplate] = useState<string>("");
    
    const [basePath, setBasePath] = useState("");
    const [targetFolder, setTargetFolder] = useState("");
    
    const [logs, setLogs] = useState<string[]>([]);
    const [isProcessing, setIsProcessing] = useState(false);

    const loadData = async () => {
        try {
            const config: any = await LoadConfig();
            if (config.settings) {
                setServerUrl(config.settings.server_url || "");
                setUsername(config.settings.username || "");
            }
            if (config.templates) {
                setTemplates(config.templates);
                const keys = Object.keys(config.templates);
                if (keys.length > 0 && !selectedTemplate) {
                    setSelectedTemplate(keys[0]);
                }
            }
            
            if (config.settings?.username) {
                try {
                    const savedPwd = await GetPassword(config.settings.username);
                    if (savedPwd) setPassword(savedPwd);
                } catch(e) {
                    // no password in keyring yet
                }
            }
        } catch (e: any) {
            setLogs(prev => [...prev, "Ошибка загрузки конфигурации: " + String(e)]);
        }
    };

    useEffect(() => {
        loadData();
    }, []);

    const handleSaveSettings = async () => {
        try {
            await SaveConfig(serverUrl, username);
            if (password) {
                await SaveCredentials(username, password);
            }
            setLogs(prev => [...prev, "✅ Настройки безопасно сохранены"]);
        } catch(e: any) {
            setLogs(prev => [...prev, "❌ Ошибка сохранения: " + e]);
        }
    }

    const handleCreate = async () => {
        if (!selectedTemplate) return;
        setIsProcessing(true);
        setLogs(["⏳ Запуск создания папок..."]);
        try {
            const pathsToCreate = templates[selectedTemplate];
            const resultLogs = await CreateStructure(serverUrl, username, password, basePath, targetFolder, pathsToCreate);
            setLogs(prev => [...prev, ...(resultLogs || [])]);
        } catch(e: any) {
            setLogs(prev => [...prev, "❌ Критическая ошибка: " + String(e)]);
        }
        setIsProcessing(false);
    }

    return (
        <div id="App">
            <div className="glass-panel">
                <h2>Настройки Nextcloud</h2>
                
                <div className="form-group">
                    <label>URL сервера</label>
                    <input type="text" value={serverUrl} onChange={e => setServerUrl(e.target.value)} placeholder="https://cloud.example.com" />
                </div>
                
                <div className="form-group">
                    <label>Имя пользователя</label>
                    <input type="text" value={username} onChange={e => setUsername(e.target.value)} placeholder="admin" />
                </div>

                <div className="form-group">
                    <label>Пароль приложения (App Password)</label>
                    <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="••••••••••••" />
                </div>
                
                <button className="btn-secondary" onClick={handleSaveSettings} style={{alignSelf: 'flex-start', marginTop: '10px'}}>
                    💾 Сохранить и обновить
                </button>

                <h3 style={{fontSize: '1rem', color: '#cbd5e1', marginBottom: 0, marginTop: '1rem'}}>Выберите базовую папку:</h3>
                <div className="tree-container">
                    {password ? (
                        <TreeNode 
                            name="/" 
                            path="" 
                            selectedPath={basePath} 
                            onSelect={setBasePath} 
                            serverUrl={serverUrl} 
                            username={username} 
                            password={password}
                            initialExpanded={true}
                        />
                    ) : (
                        <div style={{color: '#94a3b8', fontStyle: 'italic'}}>Для загрузки папок сохраните настройки и пароль</div>
                    )}
                </div>

                <div className="form-group">
                    <label>Имя новой целевой папки</label>
                    <input type="text" value={targetFolder} onChange={e => setTargetFolder(e.target.value)} placeholder="Мой Новый Проект 1" />
                </div>
            </div>

            <div className="glass-panel" style={{display: 'flex', flexDirection: 'column', height: '100%'}}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%', gap: '1rem' }}>
                    <h2 style={{ margin: 0, flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>Шаблон структуры</h2>
                    <button className="btn-secondary" onClick={loadData} style={{ flexShrink: 0 }}>↻ Обновить конфиг</button>
                </div>

                <div className="form-group">
                    <label>Выберите шаблон</label>
                    <select value={selectedTemplate} onChange={e => setSelectedTemplate(e.target.value)}>
                        {Object.keys(templates).map(key => (
                            <option key={key} value={key}>{key}</option>
                        ))}
                    </select>
                </div>

                <div className="form-group" style={{ flexGrow: 0, display: 'flex', flexDirection: 'column' }}>
                    <label>Будет создано:</label>
                    <pre className="trees-list" style={{maxHeight: '150px'}}>
                        {targetFolder ? `📁 /${basePath ? basePath + '/' : ''}${targetFolder}\n` : `📁 /${basePath}\n`}
                        {targetFolder && selectedTemplate && templates[selectedTemplate] ? (
                            templates[selectedTemplate].map(p => `   📁 ${p}\n`)
                        ) : ""}
                    </pre>
                </div>
                
                <button className="btn-primary" onClick={handleCreate} disabled={isProcessing} style={{marginTop: 'auto', marginBottom: '1rem'}}>
                    {isProcessing ? "⏳ Идет создание..." : "СОЗДАТЬ СТРУКТУРУ НА ДИСКЕ"}
                </button>

                <div className="log-box" style={{marginTop: '0', flexGrow: 1}}>
                    {logs.map((log, idx) => (
                        <div key={idx} className={`log-entry ${log.includes('Ошибка') || log.includes('✗') || log.includes('❌') ? 'log-error' : ''}`}>{log}</div>
                    ))}
                </div>
            </div>
        </div>
    )
}

export default App;
