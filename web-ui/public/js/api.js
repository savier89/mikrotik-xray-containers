// Sing-Box API Client

class APIClient {
    constructor(baseUrl, token = '') {
        this.baseUrl = baseUrl || '';
        this.token = token;
        this.connected = false;
    }

    setConfig(baseUrl, token) {
        this.baseUrl = baseUrl;
        this.token = token;
    }

    async request(endpoint, options = {}) {
        const url = `${this.baseUrl}${endpoint}`;
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers,
        };

        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        try {
            const response = await fetch(url, {
                ...options,
                headers,
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            this.connected = true;
            return data;
        } catch (error) {
            this.connected = false;
            throw error;
        }
    }

    // Health check
    async health() {
        return this.request('/api/health');
    }

    // Status
    async status() {
        return this.request('/api/status');
    }

    // Traffic stats
    async stats() {
        return this.request('/api/stats');
    }

    // Connections
    async connections() {
        return this.request('/api/connections');
    }

    // Subscriptions
    async getSubscriptions() {
        return this.request('/api/subscriptions');
    }

    async addSubscription(name, url) {
        return this.request('/api/subscriptions', {
            method: 'POST',
            body: JSON.stringify({ name, url }),
        });
    }

    async deleteSubscription(id) {
        return this.request(`/api/subscriptions/${id}`, {
            method: 'DELETE',
        });
    }

    async setActiveSubscription(id) {
        return this.request('/api/subscriptions/active', {
            method: 'POST',
            body: JSON.stringify({ id }),
        });
    }

    // Servers
    async getServers() {
        return this.request('/api/servers');
    }

    async selectServer(index) {
        return this.request('/api/servers/select', {
            method: 'POST',
            body: JSON.stringify({ index }),
        });
    }

    async testServers(timeout = 5) {
        return this.request('/api/servers/test', {
            method: 'POST',
            body: JSON.stringify({ timeout }),
        });
    }

    // Config
    async getConfig() {
        return this.request('/api/config');
    }

    async reloadConfig() {
        return this.request('/api/config', {
            method: 'POST',
        });
    }

    // Logs
    async getLogs() {
        return this.request('/api/logs');
    }
}

// Export singleton
const api = new APIClient();
