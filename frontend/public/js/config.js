// Configuration loader for the frontend application
class AppConfig {
    constructor() {
        this.config = null;
        this.loaded = false;
    }
    
    async load() {
        if (this.loaded) {
            return this.config;
        }
        
        try {
            const response = await fetch('/config/app-config.json');
            if (!response.ok) {
                throw new Error(`Failed to load config: ${response.status}`);
            }
            
            this.config = await response.json();
            this.loaded = true;
            
            // Apply environment-specific overrides
            this.applyEnvironmentOverrides();
            
            console.log('Configuration loaded successfully:', this.config);
            return this.config;
        } catch (error) {
            console.error('Error loading configuration:', error);
            
            // Fallback to default configuration
            this.config = this.getDefaultConfig();
            this.loaded = true;
            
            console.warn('Using fallback configuration');
            return this.config;
        }
    }
    
    applyEnvironmentOverrides() {
        // Override API base URL based on current location
        if (!this.config.api.baseUrl) {
            this.config.api.baseUrl = `${window.location.protocol}//${window.location.host}`;
        }
        
        // Enable debug logs in development
        if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
            this.config.development.enableDebugLogs = true;
        }
    }
    
    getDefaultConfig() {
        return {
            api: {
                baseUrl: `${window.location.protocol}//${window.location.host}`,
                version: "v1",
                timeout: 30000
            },
            auth: {
                apiKey: "demo-premium-key",
                defaultUserId: "demo-user-123"
            },
            features: {
                websocket: true,
                realTimeUpdates: true,
                notifications: true
            },
            ui: {
                theme: "light",
                refreshIntervals: {
                    metrics: 300000,
                    health: 30000,
                    websocketPing: 60000
                }
            },
            development: {
                enableDebugLogs: false,
                mockData: false
            }
        };
    }
    
    get(path) {
        if (!this.loaded) {
            console.warn('Configuration not loaded yet. Call load() first.');
            return null;
        }
        
        return this.getNestedValue(this.config, path);
    }
    
    getNestedValue(obj, path) {
        return path.split('.').reduce((current, key) => {
            return current && current[key] !== undefined ? current[key] : null;
        }, obj);
    }
    
    // Convenience methods for commonly used config values
    getApiKey() {
        return this.get('auth.apiKey');
    }
    
    getDefaultUserId() {
        return this.get('auth.defaultUserId');
    }
    
    getApiBaseUrl() {
        return this.get('api.baseUrl');
    }
    
    getApiVersion() {
        return this.get('api.version');
    }
    
    isFeatureEnabled(feature) {
        return this.get(`features.${feature}`) === true;
    }
    
    getRefreshInterval(type) {
        return this.get(`ui.refreshIntervals.${type}`) || 30000;
    }
    
    isDebugEnabled() {
        return this.get('development.enableDebugLogs') === true;
    }
}

// Create global config instance
window.appConfig = new AppConfig();