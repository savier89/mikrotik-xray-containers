// Sing-Box Manager - Main Application

class App {
    constructor() {
        this.refreshRate = 3000;
        this.refreshTimer = null;
        this.state = {
            status: null,
            stats: null,
            connections: null,
            subscriptions: [],
            servers: [],
            logs: '',
        };

        this.loadSettings();
        this.bindEvents();
        this.startRefresh();
    }

    loadSettings() {
        const settings = localStorage.getItem('singbox_settings');
        if (settings) {
            const { apiUrl, apiToken, refreshRate } = JSON.parse(settings);
            api.setConfig(apiUrl, apiToken);
            this.refreshRate = refreshRate || 3000;
            
            document.getElementById('apiUrl').value = apiUrl;
            document.getElementById('apiToken').value = apiToken;
            document.getElementById('refreshRate').value = this.refreshRate;
        }
    }

    saveSettings() {
        const apiUrl = document.getElementById('apiUrl').value;
        const apiToken = document.getElementById('apiToken').value;
        const refreshRate = parseInt(document.getElementById('refreshRate').value) || 3000;

        api.setConfig(apiUrl, apiToken);
        this.refreshRate = refreshRate;

        localStorage.setItem('singbox_settings', JSON.stringify({
            apiUrl,
            apiToken,
            refreshRate,
        }));

        this.stopRefresh();
        this.startRefresh();
    }

    bindEvents() {
        // Settings modal
        document.getElementById('settingsBtn').addEventListener('click', () => {
            document.getElementById('settingsModal').classList.add('active');
        });

        document.getElementById('closeSettingsBtn').addEventListener('click', () => {
            document.getElementById('settingsModal').classList.remove('active');
        });

        document.getElementById('modalBackdrop').addEventListener('click', () => {
            document.getElementById('settingsModal').classList.remove('active');
        });

        document.getElementById('cancelSettingsBtn').addEventListener('click', () => {
            document.getElementById('settingsModal').classList.remove('active');
        });

        document.getElementById('saveSettingsBtn').addEventListener('click', () => {
            this.saveSettings();
            document.getElementById('settingsModal').classList.remove('active');
        });

        // Add subscription modal
        document.getElementById('addSubBtn').addEventListener('click', () => {
            document.getElementById('addSubModal').classList.add('active');
        });

        document.getElementById('closeAddSubBtn').addEventListener('click', () => {
            document.getElementById('addSubModal').classList.remove('active');
        });

        document.getElementById('addSubBackdrop').addEventListener('click', () => {
            document.getElementById('addSubModal').classList.remove('active');
        });

        document.getElementById('cancelAddSubBtn').addEventListener('click', () => {
            document.getElementById('addSubModal').classList.remove('active');
        });

        document.getElementById('saveAddSubBtn').addEventListener('click', () => {
            this.addSubscription();
        });

        // Test servers
        document.getElementById('testServersBtn').addEventListener('click', () => {
            this.testServers();
        });

        // Clear logs
        document.getElementById('clearLogsBtn').addEventListener('click', () => {
            this.state.logs = '';
            this.renderLogs();
        });
    }

    async addSubscription() {
        const name = document.getElementById('subName').value;
        const url = document.getElementById('subUrl').value;

        if (!url) {
            alert('URL is required');
            return;
        }

        try {
            await api.addSubscription(name || 'Subscription', url);
            document.getElementById('addSubModal').classList.remove('active');
            document.getElementById('subName').value = '';
            document.getElementById('subUrl').value = '';
            await this.fetchData();
        } catch (error) {
            alert('Failed to add subscription: ' + error.message);
        }
    }

    async testServers() {
        try {
            const btn = document.getElementById('testServersBtn');
            btn.disabled = true;
            btn.textContent = 'TESTING...';
            
            const result = await api.testServers(5);
            await this.fetchData();
            
            btn.disabled = false;
            btn.textContent = 'TEST';
        } catch (error) {
            alert('Failed to test servers: ' + error.message);
            const btn = document.getElementById('testServersBtn');
            btn.disabled = false;
            btn.textContent = 'TEST';
        }
    }

    async fetchData() {
        try {
            // Status
            this.state.status = await api.status();
            this.updateConnectionStatus(true);
            
            // Stats
            this.state.stats = await api.stats();
            
            // Connections
            this.state.connections = await api.connections();
            
            // Subscriptions
            const subs = await api.getSubscriptions();
            this.state.subscriptions = subs.subscriptions || [];
            
            // Servers
            if (this.state.subscriptions.length > 0) {
                try {
                    const servers = await api.getServers();
                    this.state.servers = servers.servers || [];
                } catch (e) {
                    this.state.servers = [];
                }
            }
            
            // Logs
            try {
                const logs = await api.getLogs();
                this.state.logs = logs.logs || '';
            } catch (e) {
                // Ignore log errors
            }
            
            this.render();
        } catch (error) {
            this.updateConnectionStatus(false);
            console.error('Failed to fetch data:', error);
        }
    }

    updateConnectionStatus(connected) {
        const status = document.getElementById('connectionStatus');
        const dot = status.querySelector('.status-dot');
        const text = status.querySelector('.status-text');
        
        if (connected) {
            dot.className = 'status-dot connected';
            text.textContent = 'CONNECTED';
        } else {
            dot.className = 'status-dot disconnected';
            text.textContent = 'DISCONNECTED';
        }
    }

    render() {
        this.renderStatus();
        this.renderTraffic();
        this.renderConnections();
        this.renderSubscriptions();
        this.renderServers();
        this.renderLogs();
    }

    renderStatus() {
        if (!this.state.status) return;
        
        const status = this.state.status;
        document.getElementById('singboxStatus').textContent = status.running ? 'ONLINE' : 'OFFLINE';
        document.getElementById('singboxStatus').className = `panel-badge ${status.running ? 'online' : 'offline'}`;
        document.getElementById('uptime').textContent = this.formatUptime(status.uptime);
        document.getElementById('pid').textContent = status.pid || '--';
        document.getElementById('subsCount').textContent = this.state.subscriptions.length;
        document.getElementById('serversCount').textContent = this.state.servers.length;
    }

    renderTraffic() {
        if (!this.state.stats) return;
        
        const stats = this.state.stats;
        document.getElementById('upload').textContent = this.formatBytes(stats.upload || 0);
        document.getElementById('download').textContent = this.formatBytes(stats.download || 0);
    }

    renderConnections() {
        const list = document.getElementById('connectionsList');
        const count = document.getElementById('connectionsCount');
        
        if (!this.state.connections || !this.state.connections.connections) {
            list.innerHTML = '<div class="empty-state">No connection data</div>';
            count.textContent = '0';
            return;
        }
        
        const conns = this.state.connections.connections;
        count.textContent = conns.length;
        
        if (conns.length === 0) {
            list.innerHTML = '<div class="empty-state">No active connections</div>';
            return;
        }
        
        list.innerHTML = conns.slice(0, 20).map(conn => `
            <div class="conn-item">
                <span class="conn-rule">${conn.rule || 'default'}</span>
                ${conn.host || conn.process || 'unknown'}
            </div>
        `).join('');
    }

    renderSubscriptions() {
        const list = document.getElementById('subscriptionsList');
        
        if (this.state.subscriptions.length === 0) {
            list.innerHTML = '<div class="empty-state">No subscriptions</div>';
            return;
        }
        
        list.innerHTML = this.state.subscriptions.map(sub => `
            <div class="sub-item ${sub.id === this.state.status?.active_subscription ? 'active' : ''}">
                <div class="sub-info">
                    <span class="sub-name">${sub.name || 'Subscription'}</span>
                    <span class="sub-meta">${sub.server_count || 0} servers • ${sub.id}</span>
                </div>
                <div class="sub-actions">
                    <button class="btn-small ${sub.id === this.state.status?.active_subscription ? 'active' : ''}" 
                            onclick="app.setActiveSubscription('${sub.id}')">
                        ${sub.id === this.state.status?.active_subscription ? 'ACTIVE' : 'SET'}
                    </button>
                    <button class="btn-small delete" onclick="app.deleteSubscription('${sub.id}')">DEL</button>
                </div>
            </div>
        `).join('');
    }

    renderServers() {
        const list = document.getElementById('serversList');
        
        if (this.state.servers.length === 0) {
            list.innerHTML = '<div class="empty-state">No servers</div>';
            return;
        }
        
        list.innerHTML = this.state.servers.map((server, idx) => {
            const name = this.extractServerName(server);
            const latency = this.state.status?.server_stats?.[`server_${idx}`]?.latency;
            return `
                <div class="server-item ${idx === this.state.status?.selected_server ? 'selected' : ''}">
                    <div class="server-info">
                        <span class="server-name">${name}</span>
                        <span class="server-meta">Server #${idx}</span>
                        ${latency !== undefined ? `<span class="server-latency">${latency}ms</span>` : ''}
                    </div>
                    <div class="server-actions">
                        <button class="btn-small ${idx === this.state.status?.selected_server ? 'active' : ''}" 
                                onclick="app.selectServer(${idx})">
                            ${idx === this.state.status?.selected_server ? 'SELECTED' : 'SELECT'}
                        </button>
                    </div>
                </div>
            `;
        }).join('');
    }

    renderLogs() {
        const container = document.getElementById('logsContainer');
        
        if (!this.state.logs) {
            container.innerHTML = '<div class="empty-state">No logs</div>';
            return;
        }
        
        const lines = this.state.logs.split('\n').filter(l => l.trim());
        container.innerHTML = lines.slice(-50).map(line => {
            const match = line.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})/);
            const time = match ? match[1] : '';
            const message = time ? line.replace(time, '') : line;
            return `<div class="log-line"><span class="log-time">${time}</span>${message}</div>`;
        }).join('');
    }

    async setActiveSubscription(id) {
        try {
            await api.setActiveSubscription(id);
            await this.fetchData();
        } catch (error) {
            alert('Failed to set active subscription: ' + error.message);
        }
    }

    async deleteSubscription(id) {
        if (!confirm('Delete this subscription?')) return;
        
        try {
            await api.deleteSubscription(id);
            await this.fetchData();
        } catch (error) {
            alert('Failed to delete subscription: ' + error.message);
        }
    }

    async selectServer(index) {
        try {
            await api.selectServer(index);
            await this.fetchData();
        } catch (error) {
            alert('Failed to select server: ' + error.message);
        }
    }

    formatUptime(seconds) {
        if (!seconds || seconds === 0) return '--';
        
        const days = Math.floor(seconds / 86400);
        const hours = Math.floor((seconds % 86400) / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        
        if (days > 0) return `${days}d ${hours}h`;
        if (hours > 0) return `${hours}h ${minutes}m`;
        return `${minutes}m`;
    }

    formatBytes(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    }

    extractServerName(url) {
        try {
            const match = url.match(/@([^:]+)/);
            if (match) return match[1];
            
            const hashMatch = url.match(/#(.+)$/);
            if (hashMatch) return hashMatch[1];
            
            return 'Unknown';
        } catch {
            return 'Unknown';
        }
    }

    startRefresh() {
        this.fetchData();
        this.refreshTimer = setInterval(() => {
            this.fetchData();
        }, this.refreshRate);
    }

    stopRefresh() {
        if (this.refreshTimer) {
            clearInterval(this.refreshTimer);
            this.refreshTimer = null;
        }
    }
}

// Initialize app
const app = new App();
