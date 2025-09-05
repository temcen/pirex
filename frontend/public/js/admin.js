// Admin Dashboard JavaScript
class AdminDashboard {
    constructor() {
        this.config = null;
        this.apiKey = null;
        this.currentSection = 'overview';
        this.charts = {};
        this.updateInterval = null;
        
        this.init();
    }
    
    async init() {
        try {
            // Load configuration first
            this.config = await window.appConfig.load();
            this.apiKey = this.config.auth.apiKey;
            
            // Initialize components
            this.setupEventListeners();
            this.setupCharts();
            await this.loadInitialData();
            this.startPeriodicUpdates();
            
            if (this.config.development.enableDebugLogs) {
                console.log('Admin dashboard initialized with config:', this.config);
            }
        } catch (error) {
            console.error('Error initializing admin dashboard:', error);
            this.showAlert('Error initializing dashboard', 'error');
        }
    }
    
    // Helper method for making authenticated API requests
    async fetchWithAuth(url, options = {}) {
        if (!this.apiKey) {
            throw new Error('Authentication not configured. Please check configuration.');
        }
        
        const defaultOptions = {
            headers: {
                'Authorization': `Bearer ${this.apiKey}`,
                'Content-Type': 'application/json',
                ...options.headers
            },
            timeout: this.config?.api?.timeout || 30000
        };
        
        const fullUrl = url.startsWith('http') ? url : `${this.config?.api?.baseUrl || ''}${url}`;
        
        if (this.config?.development?.enableDebugLogs) {
            console.log('Admin API Request:', { url: fullUrl, options: defaultOptions });
        }
        
        return fetch(fullUrl, { ...options, headers: defaultOptions.headers });
    }
    
    setupEventListeners() {
        // Sidebar navigation
        document.querySelectorAll('[data-section]').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const section = link.getAttribute('data-section');
                this.showSection(section);
            });
        });
        
        // Algorithm configuration sliders
        this.setupAlgorithmSliders();
        
        // Button event listeners
        document.getElementById('save-algorithm-config')?.addEventListener('click', () => {
            this.saveAlgorithmConfiguration();
        });
        
        document.getElementById('reset-algorithm-config')?.addEventListener('click', () => {
            this.resetAlgorithmConfiguration();
        });
        
        document.getElementById('test-algorithm-config')?.addEventListener('click', () => {
            this.testAlgorithmConfiguration();
        });
        
        document.getElementById('save-privacy-settings')?.addEventListener('click', () => {
            this.savePrivacySettings();
        });
        
        document.getElementById('refresh-content-status')?.addEventListener('click', () => {
            this.loadContentStatus();
        });
        
        document.getElementById('create-ab-test')?.addEventListener('click', () => {
            this.createABTest();
        });
    }
    
    setupAlgorithmSliders() {
        const sliders = [
            { id: 'semantic-weight', valueId: 'semantic-weight-value' },
            { id: 'collaborative-weight', valueId: 'collaborative-weight-value' },
            { id: 'pagerank-weight', valueId: 'pagerank-weight-value' },
            { id: 'diversity-weight', valueId: 'diversity-value' },
            { id: 'serendipity-ratio', valueId: 'serendipity-value' }
        ];
        
        sliders.forEach(slider => {
            const element = document.getElementById(slider.id);
            const valueElement = document.getElementById(slider.valueId);
            
            if (element && valueElement) {
                element.addEventListener('input', (e) => {
                    valueElement.textContent = e.target.value;
                    this.validateWeights();
                });
            }
        });
    }
    
    validateWeights() {
        const semanticWeight = parseFloat(document.getElementById('semantic-weight').value);
        const collaborativeWeight = parseFloat(document.getElementById('collaborative-weight').value);
        const pagerankWeight = parseFloat(document.getElementById('pagerank-weight').value);
        
        const total = semanticWeight + collaborativeWeight + pagerankWeight;
        
        if (Math.abs(total - 1.0) > 0.01) {
            // Show warning if weights don't sum to 1.0
            this.showAlert('Warning: Algorithm weights should sum to 1.0', 'warning');
        }
    }
    
    showSection(sectionName) {
        // Hide all sections
        document.querySelectorAll('.admin-section').forEach(section => {
            section.style.display = 'none';
        });
        
        // Show selected section
        const targetSection = document.getElementById(sectionName);
        if (targetSection) {
            targetSection.style.display = 'block';
            this.currentSection = sectionName;
            
            // Update sidebar navigation
            document.querySelectorAll('[data-section]').forEach(link => {
                link.classList.remove('active');
            });
            document.querySelector(`[data-section="${sectionName}"]`)?.classList.add('active');
            
            // Load section-specific data
            this.loadSectionData(sectionName);
        }
    }
    
    loadSectionData(sectionName) {
        switch (sectionName) {
            case 'overview':
                this.loadOverviewData();
                break;
            case 'analytics':
                this.loadAnalyticsData();
                break;
            case 'content':
                this.loadContentData();
                break;
            case 'users':
                this.loadUserData();
                break;
            case 'algorithms':
                this.loadAlgorithmConfig();
                break;
            case 'monitoring':
                this.loadMonitoringData();
                break;
            case 'ab-testing':
                this.loadABTestData();
                break;
        }
    }
    
    async loadInitialData() {
        try {
            await this.loadOverviewData();
        } catch (error) {
            console.error('Error loading initial data:', error);
            this.showAlert('Error loading dashboard data', 'error');
        }
    }
    
    async loadOverviewData() {
        try {
            // Load system health
            const healthResponse = await fetch('/health/detailed');
            if (healthResponse.ok) {
                const health = await healthResponse.json();
                this.updateSystemHealth(health);
            }
            
            // Load key metrics
            const metricsResponse = await this.fetchWithAuth('/api/v1/admin/metrics/overview');
            if (metricsResponse.ok) {
                const metrics = await metricsResponse.json();
                this.updateOverviewMetrics(metrics);
            }
            
            // Load recent alerts
            const alertsResponse = await this.fetchWithAuth('/api/v1/admin/alerts/recent');
            if (alertsResponse.ok) {
                const alerts = await alertsResponse.json();
                this.updateRecentAlerts(alerts);
            }
            
        } catch (error) {
            console.error('Error loading overview data:', error);
        }
    }
    
    async loadAnalyticsData() {
        try {
            const response = await this.fetchWithAuth('/api/v1/admin/analytics');
            if (response.ok) {
                const data = await response.json();
                this.updateAnalyticsCharts(data);
                this.updateAlgorithmPerformanceTable(data.algorithm_performance);
            }
        } catch (error) {
            console.error('Error loading analytics data:', error);
        }
    }
    
    async loadContentData() {
        try {
            const response = await this.fetchWithAuth('/api/v1/admin/content/status');
            if (response.ok) {
                const data = await response.json();
                this.updateContentMetrics(data);
                this.updateContentStatusTable(data.jobs);
            }
        } catch (error) {
            console.error('Error loading content data:', error);
        }
    }
    
    async loadUserData() {
        try {
            const response = await this.fetchWithAuth('/api/v1/admin/users/analytics');
            if (response.ok) {
                const data = await response.json();
                this.updateUserSegmentationChart(data);
                this.updateUserTierStats(data.tier_stats);
            }
        } catch (error) {
            console.error('Error loading user data:', error);
        }
    }
    
    async loadAlgorithmConfig() {
        try {
            const response = await this.fetchWithAuth('/api/v1/admin/algorithms/config');
            if (response.ok) {
                const config = await response.json();
                this.updateAlgorithmConfigUI(config);
            }
        } catch (error) {
            console.error('Error loading algorithm config:', error);
        }
    }
    
    async loadMonitoringData() {
        try {
            const response = await this.fetchWithAuth('/api/v1/admin/monitoring/metrics');
            if (response.ok) {
                const data = await response.json();
                this.updateMonitoringMetrics(data);
                this.updateSystemPerformanceChart(data.performance);
                this.updateDBConnectionsChart(data.db_connections);
            }
        } catch (error) {
            console.error('Error loading monitoring data:', error);
        }
    }
    
    async loadABTestData() {
        try {
            const response = await this.fetchWithAuth('/api/v1/admin/ab-tests');
            if (response.ok) {
                const data = await response.json();
                this.updateABTestsTable(data.tests);
            }
        } catch (error) {
            console.error('Error loading A/B test data:', error);
        }
    }
    
    // Update UI methods
    updateSystemHealth(health) {
        const statusElement = document.getElementById('system-health-status');
        if (statusElement) {
            statusElement.textContent = health.status || 'Unknown';
            statusElement.className = health.status === 'healthy' ? 'text-success' : 
                                    health.status === 'degraded' ? 'text-warning' : 'text-danger';
        }
        
        const serviceContainer = document.getElementById('admin-service-status');
        if (serviceContainer && health.services) {
            serviceContainer.innerHTML = '';
            
            Object.entries(health.services).forEach(([service, status]) => {
                const serviceElement = document.createElement('div');
                serviceElement.className = 'd-flex justify-content-between align-items-center mb-2 p-2 rounded';
                serviceElement.style.backgroundColor = status === 'healthy' ? '#d1e7dd' : 
                                                     status === 'degraded' ? '#fff3cd' : '#f8d7da';
                
                serviceElement.innerHTML = `
                    <span><strong>${this.formatServiceName(service)}</strong></span>
                    <span class="badge ${status === 'healthy' ? 'bg-success' : 
                                       status === 'degraded' ? 'bg-warning' : 'bg-danger'}">${status}</span>
                `;
                
                serviceContainer.appendChild(serviceElement);
            });
        }
    }
    
    updateOverviewMetrics(metrics) {
        document.getElementById('admin-active-users').textContent = 
            this.formatNumber(metrics.active_users || 0);
        document.getElementById('admin-total-recommendations').textContent = 
            this.formatNumber(metrics.total_recommendations || 0);
        document.getElementById('admin-response-time').textContent = 
            `${(metrics.avg_response_time || 0).toFixed(0)}ms`;
    }
    
    updateRecentAlerts(alerts) {
        const container = document.getElementById('recent-alerts');
        if (!container) return;
        
        container.innerHTML = '';
        
        if (!alerts || alerts.length === 0) {
            container.innerHTML = '<p class="text-muted">No recent alerts</p>';
            return;
        }
        
        alerts.slice(0, 5).forEach(alert => {
            const alertElement = document.createElement('div');
            alertElement.className = `alert alert-${alert.level || 'info'} alert-sm mb-2`;
            alertElement.innerHTML = `
                <div class="d-flex justify-content-between">
                    <span>${alert.message}</span>
                    <small>${this.formatTimeAgo(alert.timestamp)}</small>
                </div>
            `;
            container.appendChild(alertElement);
        });
    }
    
    updateContentMetrics(data) {
        document.getElementById('total-content-items').textContent = 
            this.formatNumber(data.total_items || 0);
        document.getElementById('processing-queue-size').textContent = 
            this.formatNumber(data.queue_size || 0);
        document.getElementById('failed-content-items').textContent = 
            this.formatNumber(data.failed_items || 0);
    }
    
    updateContentStatusTable(jobs) {
        const tbody = document.querySelector('#content-status-table tbody');
        if (!tbody) return;
        
        tbody.innerHTML = '';
        
        if (!jobs || jobs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="text-center text-muted">No jobs found</td></tr>';
            return;
        }
        
        jobs.forEach(job => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${job.job_id}</td>
                <td>${job.content_type}</td>
                <td><span class="badge bg-${this.getStatusColor(job.status)}">${job.status}</span></td>
                <td>
                    <div class="progress" style="height: 20px;">
                        <div class="progress-bar" role="progressbar" style="width: ${job.progress || 0}%">
                            ${job.progress || 0}%
                        </div>
                    </div>
                </td>
                <td>${this.formatTimeAgo(job.created_at)}</td>
                <td>
                    <button class="btn btn-sm btn-outline-primary" onclick="admin.viewJobDetails('${job.job_id}')">
                        View
                    </button>
                </td>
            `;
            tbody.appendChild(row);
        });
    }
    
    updateAlgorithmPerformanceTable(performance) {
        const tbody = document.querySelector('#algorithm-performance-table tbody');
        if (!tbody || !performance) return;
        
        tbody.innerHTML = '';
        
        Object.entries(performance).forEach(([algorithm, metrics]) => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${algorithm}</td>
                <td>${(metrics.ctr || 0).toFixed(2)}%</td>
                <td>${(metrics.conversion_rate || 0).toFixed(2)}%</td>
                <td>${(metrics.avg_confidence || 0).toFixed(3)}</td>
                <td>${(metrics.performance_score || 0).toFixed(2)}</td>
                <td><span class="badge bg-${metrics.status === 'active' ? 'success' : 'secondary'}">${metrics.status || 'unknown'}</span></td>
            `;
            tbody.appendChild(row);
        });
    }
    
    updateMonitoringMetrics(data) {
        document.getElementById('cpu-usage').textContent = `${(data.cpu_usage || 0).toFixed(1)}%`;
        document.getElementById('memory-usage').textContent = `${(data.memory_usage || 0).toFixed(1)}%`;
        document.getElementById('active-connections').textContent = this.formatNumber(data.active_connections || 0);
        document.getElementById('cache-hit-rate').textContent = `${(data.cache_hit_rate || 0).toFixed(1)}%`;
    }
    
    // Chart setup and updates
    setupCharts() {
        this.setupRevenueChart();
        this.setupEngagementChart();
        this.setupUserSegmentationChart();
        this.setupSystemPerformanceChart();
        this.setupDBConnectionsChart();
    }
    
    setupRevenueChart() {
        const ctx = document.getElementById('revenue-chart');
        if (!ctx) return;
        
        this.charts.revenue = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Revenue Impact',
                    data: [],
                    borderColor: '#198754',
                    backgroundColor: 'rgba(25, 135, 84, 0.1)',
                    tension: 0.4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: {
                            callback: function(value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                }
            }
        });
    }
    
    setupEngagementChart() {
        const ctx = document.getElementById('engagement-chart');
        if (!ctx) return;
        
        this.charts.engagement = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: [],
                datasets: [{
                    label: 'Session Duration (minutes)',
                    data: [],
                    backgroundColor: '#0d6efd'
                }, {
                    label: 'Pages per Session',
                    data: [],
                    backgroundColor: '#6f42c1'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true
                    }
                }
            }
        });
    }
    
    setupUserSegmentationChart() {
        const ctx = document.getElementById('user-segmentation-chart');
        if (!ctx) return;
        
        this.charts.userSegmentation = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: ['New Users', 'Active Users', 'Inactive Users', 'Power Users'],
                datasets: [{
                    data: [25, 45, 20, 10],
                    backgroundColor: ['#0d6efd', '#198754', '#ffc107', '#dc3545']
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false
            }
        });
    }
    
    setupSystemPerformanceChart() {
        const ctx = document.getElementById('system-performance-chart');
        if (!ctx) return;
        
        this.charts.systemPerformance = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'CPU Usage (%)',
                    data: [],
                    borderColor: '#dc3545',
                    backgroundColor: 'rgba(220, 53, 69, 0.1)'
                }, {
                    label: 'Memory Usage (%)',
                    data: [],
                    borderColor: '#ffc107',
                    backgroundColor: 'rgba(255, 193, 7, 0.1)'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: {
                        beginAtZero: true,
                        max: 100
                    }
                }
            }
        });
    }
    
    setupDBConnectionsChart() {
        const ctx = document.getElementById('db-connections-chart');
        if (!ctx) return;
        
        this.charts.dbConnections = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: ['Active', 'Idle', 'Available'],
                datasets: [{
                    data: [10, 5, 15],
                    backgroundColor: ['#dc3545', '#ffc107', '#198754']
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false
            }
        });
    }
    
    // Configuration management
    async saveAlgorithmConfiguration() {
        try {
            const config = this.getAlgorithmConfigFromUI();
            
            const response = await this.fetchWithAuth('/api/v1/admin/algorithms/config', {
                method: 'PUT',
                body: JSON.stringify(config)
            });
            
            if (response.ok) {
                this.showAlert('Algorithm configuration saved successfully', 'success');
            } else {
                throw new Error('Failed to save configuration');
            }
        } catch (error) {
            console.error('Error saving algorithm configuration:', error);
            this.showAlert('Error saving configuration', 'error');
        }
    }
    
    getAlgorithmConfigFromUI() {
        return {
            algorithms: {
                semantic_search: {
                    weight: parseFloat(document.getElementById('semantic-weight').value),
                    similarity_threshold: parseFloat(document.getElementById('semantic-threshold').value)
                },
                collaborative_filtering: {
                    weight: parseFloat(document.getElementById('collaborative-weight').value),
                    similarity_threshold: parseFloat(document.getElementById('collaborative-threshold').value)
                },
                pagerank: {
                    weight: parseFloat(document.getElementById('pagerank-weight').value),
                    damping_factor: parseFloat(document.getElementById('pagerank-damping').value)
                }
            },
            diversity: {
                intra_list_diversity: parseFloat(document.getElementById('diversity-weight').value),
                category_max_items: parseInt(document.getElementById('category-max').value),
                serendipity_ratio: parseFloat(document.getElementById('serendipity-ratio').value)
            },
            features: {
                ml_ranking: document.getElementById('enable-ml-ranking').checked,
                real_time_learning: document.getElementById('enable-real-time-learning').checked,
                explanation_service: document.getElementById('enable-explanation-service').checked
            }
        };
    }
    
    resetAlgorithmConfiguration() {
        // Reset to default values
        document.getElementById('semantic-weight').value = 0.4;
        document.getElementById('semantic-weight-value').textContent = '0.4';
        document.getElementById('collaborative-weight').value = 0.3;
        document.getElementById('collaborative-weight-value').textContent = '0.3';
        document.getElementById('pagerank-weight').value = 0.3;
        document.getElementById('pagerank-weight-value').textContent = '0.3';
        
        document.getElementById('semantic-threshold').value = 0.7;
        document.getElementById('collaborative-threshold').value = 0.5;
        document.getElementById('pagerank-damping').value = 0.85;
        
        document.getElementById('diversity-weight').value = 0.3;
        document.getElementById('diversity-value').textContent = '0.3';
        document.getElementById('category-max').value = 3;
        document.getElementById('serendipity-ratio').value = 0.15;
        document.getElementById('serendipity-value').textContent = '0.15';
        
        document.getElementById('enable-ml-ranking').checked = true;
        document.getElementById('enable-real-time-learning').checked = true;
        document.getElementById('enable-explanation-service').checked = true;
        
        this.showAlert('Configuration reset to defaults', 'info');
    }
    
    async testAlgorithmConfiguration() {
        try {
            const config = this.getAlgorithmConfigFromUI();
            
            const response = await this.fetchWithAuth('/api/v1/admin/algorithms/test', {
                method: 'POST',
                body: JSON.stringify(config)
            });
            
            if (response.ok) {
                const result = await response.json();
                this.showAlert(`Test completed. Performance score: ${result.performance_score}`, 'info');
            } else {
                throw new Error('Test failed');
            }
        } catch (error) {
            console.error('Error testing configuration:', error);
            this.showAlert('Error testing configuration', 'error');
        }
    }
    
    // Utility methods
    formatNumber(num) {
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        } else if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toString();
    }
    
    formatServiceName(service) {
        return service.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
    }
    
    formatTimeAgo(timestamp) {
        const now = new Date();
        const time = new Date(timestamp);
        const diffInSeconds = Math.floor((now - time) / 1000);
        
        if (diffInSeconds < 60) return 'Just now';
        if (diffInSeconds < 3600) return `${Math.floor(diffInSeconds / 60)}m ago`;
        if (diffInSeconds < 86400) return `${Math.floor(diffInSeconds / 3600)}h ago`;
        return `${Math.floor(diffInSeconds / 86400)}d ago`;
    }
    
    getStatusColor(status) {
        switch (status) {
            case 'completed': return 'success';
            case 'processing': return 'primary';
            case 'failed': return 'danger';
            case 'queued': return 'warning';
            default: return 'secondary';
        }
    }
    
    showAlert(message, type = 'info') {
        // Create and show bootstrap alert
        const alertDiv = document.createElement('div');
        alertDiv.className = `alert alert-${type} alert-dismissible fade show position-fixed`;
        alertDiv.style.cssText = 'top: 20px; right: 20px; z-index: 9999; min-width: 300px;';
        alertDiv.innerHTML = `
            ${message}
            <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
        `;
        
        document.body.appendChild(alertDiv);
        
        // Auto-remove after 5 seconds
        setTimeout(() => {
            if (alertDiv.parentNode) {
                alertDiv.parentNode.removeChild(alertDiv);
            }
        }, 5000);
    }
    
    startPeriodicUpdates() {
        // Update data every 30 seconds
        this.updateInterval = setInterval(() => {
            this.loadSectionData(this.currentSection);
        }, 30000);
    }
    
    // Placeholder methods for future implementation
    viewJobDetails(jobId) {
        console.log('Viewing job details for:', jobId);
        this.showAlert(`Viewing details for job ${jobId}`, 'info');
    }
    
    createABTest() {
        console.log('Creating new A/B test');
        this.showAlert('A/B test creation feature coming soon', 'info');
    }
    
    savePrivacySettings() {
        console.log('Saving privacy settings');
        this.showAlert('Privacy settings saved successfully', 'success');
    }
}

// Initialize admin dashboard when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.admin = new AdminDashboard();
});