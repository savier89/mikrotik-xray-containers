// Main Application
class App {
    constructor() {
        this.refreshRate = 3000;
        this.refreshTimer = null;
        this.currentPage = 'dashboard';
        this.state = {
            status: null,
            stats: null,
            connections: null,
            subscriptions: [],
            servers: [],
            config: null,
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
            api.baseUrl = apiUrl;
            api.token = apiToken;
            this.refreshRate = refreshRate || 3000;
            
            document.getElementById('apiUrl').value = apiUrl;
            document.getElementById('apiToken').value = apiToken;
            document.getElementById('refreshRate').value = this.refreshRate;
            document.getElementById('refreshRateValue').textContent = `${this.refreshRate}ms`;
        }
    }

    saveSettings() {
        const apiUrl = document.getElementById('apiUrl').value;
        const apiToken = document.getElementById('apiToken').value;
        const refreshRate = parseInt(document.getElementById('refreshRate').value) || 3000;

        api.baseUrl = apiUrl;
        api.token = apiToken;
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
        // Navigation
        document.querySelectorAll('.nav-item').forEach(item => {
            item.addEventListener('click', (e) => {
                e.preventDefault();
                const page = item.dataset.page;
                this.switchPage(page);
            });
        });

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

        // Refresh rate slider
        document.getElementById('refreshRate').addEventListener('input', (e) => {
            document.getElementById('refreshRateValue').textContent = `${e.target.value}ms`;
        });

        // Add subscription modal
        document.getElementById('addSubBtn').addEventListener('click', () => {
            document.getElementById('addSubModal').classList.add('active');
        });

        document.getElementById('addSubBtn2').addEventListener('click', () => {
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

        // Quick actions
        document.getElementById('refreshBtn').addEventListener('click', () => {
            this.fetchData();
        });

        document.getElementById('testServersBtn').addEventListener('click', () => {
            this.testServers();
        });

        document.getElementById('testServersBtn2').addEventListener('click', () => {
            this.testServers();
        });

        document.getElementById('reloadConfigBtn').addEventListener('click', () => {
            this.reloadConfig();
        });

        // Config tabs
        document.querySelectorAll('.tab').forEach(tab => {
            tab.addEventListener('click', () => {
                const tabId = tab.dataset.tab;
                this.switchTab(tabId);
            });
        });

        // Clear logs
        document.getElementById('clearLogsBtn').addEventListener('click', () => {
            this.state.logs = '';
            this.renderLogs();
        });
    }

    switchPage(page) {
        this.currentPage = page;
        
        // Update nav
        document.querySelectorAll('.nav-item').forEach(item => {
            item.classList.toggle('active', item.dataset.page === page);
        });
        
        // Update pages
        document.querySelectorAll('.page').forEach(p => {
            p.classList.toggle('active', p.id === `page-${page}`);
        });
    }

    switchTab(tabId) {
        document.querySelectorAll('.tab').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.tab === tabId);
        });
        
        document.querySelectorAll('.tab-content').forEach(content => {
            content.classList.toggle('active', content.id === `tab-${tabId}`);
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
            await api.testServers(5);
            await this.fetchData();
        } catch (error) {
            alert('Failed to test servers: ' + error.message);
        }
    }

    async reloadConfig() {
        try {
            await api.reloadConfig();
            await this.fetchData();
        } catch (error) {
            alert('Failed to reload config: ' + error.message);
        }
    }

    async fetchData() {
        try {
            this.state.status = await api.status();
            this.state.stats = await api.stats();
            this.state.connections = await api.connections();
            
            const subs = await api.getSubscriptions();
            this.state.subscriptions = subs.subscriptions || [];
            
            if (this.state.subscriptions.length > 0) {
                try {
                    const servers = await api.getServers();
                    this.state.servers = servers.servers || [];
                } catch (e) {
                    this.state.servers = [];
                }
            }
            
            try {
                this.state.config = await api.getConfig();
            } catch (e) {
                this.state.config = null;
            }
            
            try {
                const logs = await api.getLogs();
                this.state.logs = logs.logs || '';
            } catch (e) {
                // Ignore log errors
            }
            
            this.updateConnectionStatus(true);
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
        this.renderDashboard();
        this.renderSubscriptions();
        this.renderServers();
        this.renderConfig();
        this.renderLogs();
    }

    renderDashboard() {
        if (!this.state.status) return;
        
        const status = this.state.status;
        document.getElementById('singboxStatus').textContent = status.running ? 'ONLINE' : 'OFFLINE';
        document.getElementById('singboxStatus').className = `badge ${status.running ? 'online' : 'offline'}`;
        document.getElementById('uptime').textContent = this.formatUptime(status.uptime);
        document.getElementById('pid').textContent = status.pid || '--';
        document.getElementById('subsCount').textContent = this.state.subscriptions.length;
        document.getElementById('serversCount').textContent = this.state.servers.length;

        if (this.state.stats) {
            document.getElementById('upload').textContent = this.formatBytes(this.state.stats.upload || 0);
            document.getElementById('download').textContent = this.formatBytes(this.state.stats.download || 0);
        }
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

    renderConfig() {
        if (!this.state.config) return;
        
        const config = this.state.config;
        
        // Preview
        document.getElementById('configPreview').innerHTML = `
            <div class="config-section">
                <h4>Inbounds</h4>
                <pre>${JSON.stringify(config.inbounds, null, 2)}</pre>
            </div>
            <div class="config-section">
                <h4>Outbounds</h4>
                <pre>${JSON.stringify(config.outbounds, null, 2)}</pre>
            </div>
        `;
        
        // Raw JSON
        document.getElementById('configRaw').textContent = JSON.stringify(config, null, 2);
        
        // Subscription
        if (this.state.subscriptions.length > 0) {
            const activeSub = this.state.subscriptions.find(s => s.id === this.state.status?.active_subscription);
            document.getElementById('subscriptionConfig').innerHTML = `
                <div class="sub-config">
                    <h4>${activeSub?.name || 'Active Subscription'}</h4>
                    <p>${activeSub?.url || ''}</p>
                    <p>${this.state.servers.length} servers</p>
                </div>
            `;
        }
    }

    renderLogs() {
        const container = document.getElementById('logsContent');
        const count = document.getElementById('logCount');
        
        if (!this.state.logs) {
            container.innerHTML = '<div class="empty-state">No logs</div>';
            count.textContent = '0 lines';
            return;
        }
        
        const lines = this.state.logs.split('\n').filter(l => l.trim());
        count.textContent = `${lines.length} lines`;
        
        container.innerHTML = lines.slice(-100).map(line => {
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