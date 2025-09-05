// Dashboard JavaScript
class RecommendationDashboard {
    constructor() {
        this.ws = null;
        this.config = null;
        this.currentUser = null;
        this.apiKey = null;
        this.currentSection = 'dashboard';
        this.recommendations = [];
        this.charts = {};
        this.metrics = {};
        
        this.init();
    }
    
    async init() {
        try {
            // Load configuration first
            this.config = await window.appConfig.load();
            this.currentUser = this.config.auth.defaultUserId;
            this.apiKey = this.config.auth.apiKey;
            
            // Initialize components
            this.setupWebSocket();
            this.setupEventListeners();
            this.setupCharts();
            await this.loadInitialData();
            this.startPeriodicUpdates();
            
            if (this.config.development.enableDebugLogs) {
                console.log('Dashboard initialized with config:', this.config);
            }
        } catch (error) {
            console.error('Error initializing dashboard:', error);
            this.showNotification('Error initializing dashboard', 'error');
        }
    }
    
    // Helper method for making authenticated API requests
    async fetchWithAuth(url, options = {}) {
        if (!this.apiKey || !this.currentUser) {
            throw new Error('Authentication not configured. Please check configuration.');
        }
        
        const defaultOptions = {
            headers: {
                'Authorization': `Bearer ${this.apiKey}`,
                'X-User-ID': this.currentUser,
                'Content-Type': 'application/json',
                ...options.headers
            },
            timeout: this.config?.api?.timeout || 30000
        };
        
        const fullUrl = url.startsWith('http') ? url : `${this.config?.api?.baseUrl || ''}${url}`;
        
        if (this.config?.development?.enableDebugLogs) {
            console.log('API Request:', { url: fullUrl, options: defaultOptions });
        }
        
        return fetch(fullUrl, { ...options, headers: defaultOptions.headers });
    }
    
    // WebSocket Setup
    setupWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}`;
        
        this.connectWebSocket(wsUrl);
    }
    
    connectWebSocket(url) {
        try {
            this.ws = new WebSocket(url);
            
            this.ws.onopen = () => {
                console.log('WebSocket connected');
                this.updateConnectionStatus('connected');
                
                // Subscribe to user-specific updates
                this.ws.send(JSON.stringify({
                    type: 'subscribe',
                    userId: this.currentUser
                }));
            };
            
            this.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleWebSocketMessage(data);
                } catch (error) {
                    console.error('Error parsing WebSocket message:', error);
                }
            };
            
            this.ws.onclose = () => {
                console.log('WebSocket disconnected');
                this.updateConnectionStatus('disconnected');
                
                // Attempt to reconnect after 5 seconds
                setTimeout(() => {
                    this.connectWebSocket(url);
                }, 5000);
            };
            
            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                this.updateConnectionStatus('disconnected');
            };
            
        } catch (error) {
            console.error('Error creating WebSocket connection:', error);
            this.updateConnectionStatus('disconnected');
        }
    }
    
    handleWebSocketMessage(data) {
        switch (data.type) {
            case 'recommendation_update':
                this.handleRecommendationUpdate(data);
                break;
            case 'metrics_update':
                this.handleMetricsUpdate(data);
                break;
            case 'system_status':
                this.handleSystemStatus(data);
                break;
            case 'notification':
                this.showNotification(data.message, data.level || 'info');
                break;
            default:
                console.log('Unknown WebSocket message type:', data.type);
        }
    }
    
    updateConnectionStatus(status) {
        const statusElement = document.getElementById('connection-status');
        const textElement = document.getElementById('connection-text');
        
        statusElement.className = `bi bi-circle-fill text-${status === 'connected' ? 'success' : status === 'connecting' ? 'warning' : 'danger'}`;
        textElement.textContent = status.charAt(0).toUpperCase() + status.slice(1);
    }
    
    // Event Listeners
    setupEventListeners() {
        // Navigation
        document.querySelectorAll('.nav-link').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const section = link.getAttribute('href').substring(1);
                if (section !== 'admin.html') {
                    this.showSection(section);
                }
            });
        });
        
        // Recommendations
        document.getElementById('refresh-recommendations')?.addEventListener('click', () => {
            this.loadRecommendations();
        });
        
        document.getElementById('filter-toggle')?.addEventListener('click', () => {
            this.toggleFilters();
        });
        
        document.getElementById('apply-filters')?.addEventListener('click', () => {
            this.applyFilters();
        });
        
        document.getElementById('load-more-recommendations')?.addEventListener('click', () => {
            this.loadMoreRecommendations();
        });
        
        // Profile
        document.getElementById('preferences-form')?.addEventListener('submit', (e) => {
            e.preventDefault();
            this.savePreferences();
        });
        
        document.getElementById('view-history')?.addEventListener('click', () => {
            this.viewRecommendationHistory();
        });
        
        // Search
        document.getElementById('search-input')?.addEventListener('input', (e) => {
            this.debounce(() => this.searchRecommendations(e.target.value), 300);
        });
    }
    
    // Section Management
    showSection(sectionName) {
        // Hide all sections
        document.querySelectorAll('.content-section').forEach(section => {
            section.style.display = 'none';
        });
        
        // Show selected section
        const targetSection = document.getElementById(sectionName);
        if (targetSection) {
            targetSection.style.display = 'block';
            this.currentSection = sectionName;
            
            // Update navigation
            document.querySelectorAll('.nav-link').forEach(link => {
                link.classList.remove('active');
            });
            document.querySelector(`[href="#${sectionName}"]`)?.classList.add('active');
            
            // Load section-specific data
            this.loadSectionData(sectionName);
        }
    }
    
    loadSectionData(sectionName) {
        switch (sectionName) {
            case 'dashboard':
                this.loadDashboardData();
                break;
            case 'recommendations':
                this.loadRecommendations();
                break;
            case 'profile':
                this.loadProfileData();
                break;
        }
    }
    
    // Data Loading
    async loadInitialData() {
        try {
            await Promise.all([
                this.loadDashboardData(),
                this.loadHealthStatus()
            ]);
        } catch (error) {
            console.error('Error loading initial data:', error);
            this.showNotification('Error loading dashboard data', 'error');
        }
    }
    
    async loadDashboardData() {
        try {
            // Load business metrics
            const metricsResponse = await this.fetchWithAuth('/api/v1/metrics/business');
            if (metricsResponse.ok) {
                const metrics = await metricsResponse.json();
                this.updateMetricsDisplay(metrics);
            }
            
            // Load performance data for charts
            const performanceResponse = await this.fetchWithAuth('/api/v1/metrics/performance');
            if (performanceResponse.ok) {
                const performance = await performanceResponse.json();
                this.updatePerformanceCharts(performance);
            }
            
        } catch (error) {
            console.error('Error loading dashboard data:', error);
        }
    }
    
    async loadHealthStatus() {
        try {
            const response = await fetch('/health/detailed');
            if (response.ok) {
                const health = await response.json();
                this.updateHealthDisplay(health);
            }
        } catch (error) {
            console.error('Error loading health status:', error);
        }
    }
    
    async loadRecommendations() {
        try {
            this.setLoadingState('recommendations-grid', true);
            
            const response = await this.fetchWithAuth(`/api/v1/recommendations/${this.currentUser}?count=20&explain=true`);
            if (response.ok) {
                const data = await response.json();
                this.recommendations = data.recommendations || [];
                this.displayRecommendations(this.recommendations);
            } else {
                throw new Error('Failed to load recommendations');
            }
        } catch (error) {
            console.error('Error loading recommendations:', error);
            this.showNotification('Error loading recommendations', 'error');
        } finally {
            this.setLoadingState('recommendations-grid', false);
        }
    }
    
    async loadProfileData() {
        try {
            // Note: User profile endpoint doesn't exist yet, so we'll skip this for now
            // const response = await fetch(`/api/v1/users/${this.currentUser}/profile`);
            // if (response.ok) {
            //     const profile = await response.json();
            //     this.updateProfileDisplay(profile);
            // }
            
            // Load recent interactions
            const interactionsResponse = await this.fetchWithAuth(`/api/v1/users/${this.currentUser}/interactions?limit=10`);
            if (interactionsResponse.ok) {
                const interactions = await interactionsResponse.json();
                this.displayRecentInteractions(interactions);
            }
        } catch (error) {
            console.error('Error loading profile data:', error);
        }
    }
    
    // Display Updates
    updateMetricsDisplay(metrics) {
        document.getElementById('total-recommendations').textContent = 
            this.formatNumber(metrics.total_recommendations || 0);
        document.getElementById('ctr-rate').textContent = 
            `${(metrics.click_through_rate || 0).toFixed(1)}%`;
        document.getElementById('conversion-rate').textContent = 
            `${(metrics.conversion_rate || 0).toFixed(1)}%`;
        document.getElementById('active-users').textContent = 
            this.formatNumber(metrics.active_users || 0);
        
        // Add animation to updated metrics
        document.querySelectorAll('[id$="-recommendations"], [id$="-rate"], [id$="-users"]').forEach(el => {
            el.classList.add('metric-update');
            setTimeout(() => el.classList.remove('metric-update'), 500);
        });
    }
    
    updateHealthDisplay(health) {
        const container = document.getElementById('health-status');
        if (!container) return;
        
        container.innerHTML = '';
        
        if (health.services) {
            Object.entries(health.services).forEach(([service, status]) => {
                const serviceElement = document.createElement('div');
                serviceElement.className = `col-md-3 mb-2`;
                
                const statusClass = status === 'healthy' ? 'healthy' : 
                                  status === 'degraded' ? 'degraded' : 'unhealthy';
                const iconClass = status === 'healthy' ? 'bi-check-circle-fill' : 
                                status === 'degraded' ? 'bi-exclamation-triangle-fill' : 'bi-x-circle-fill';
                
                serviceElement.innerHTML = `
                    <div class="health-service ${statusClass}">
                        <div>
                            <strong>${this.formatServiceName(service)}</strong>
                            <div class="small text-muted">${status}</div>
                        </div>
                        <i class="bi ${iconClass} health-status-icon ${statusClass}"></i>
                    </div>
                `;
                
                container.appendChild(serviceElement);
            });
        }
    }
    
    displayRecommendations(recommendations) {
        const container = document.getElementById('recommendations-grid');
        if (!container) return;
        
        container.innerHTML = '';
        
        recommendations.forEach((rec, index) => {
            const recElement = document.createElement('div');
            recElement.className = 'col-md-4 col-lg-3 mb-4';
            
            recElement.innerHTML = `
                <div class="recommendation-item" data-item-id="${rec.item_id}">
                    <img src="${rec.image_url || 'https://via.placeholder.com/300x200'}" 
                         alt="${rec.title}" class="recommendation-image">
                    <h6 class="recommendation-title">${rec.title}</h6>
                    <p class="recommendation-description">${rec.description || ''}</p>
                    <div class="recommendation-meta">
                        <span class="recommendation-price">$${rec.price || '0.00'}</span>
                        <div class="recommendation-rating">
                            ${this.generateStarRating(rec.rating || 0)}
                        </div>
                    </div>
                    <div class="recommendation-actions">
                        <button class="btn btn-outline-primary btn-sm like-btn" data-action="like">
                            <i class="bi bi-heart"></i>
                        </button>
                        <button class="btn btn-primary btn-sm view-btn" data-action="view">
                            View Details
                        </button>
                        <button class="btn btn-outline-secondary btn-sm dislike-btn" data-action="dislike">
                            <i class="bi bi-heart-slash"></i>
                        </button>
                    </div>
                    ${rec.explanation ? `
                        <div class="explanation-tooltip mt-2">
                            <i class="bi bi-info-circle text-muted"></i>
                            <span class="tooltip-text">${rec.explanation}</span>
                        </div>
                    ` : ''}
                </div>
            `;
            
            container.appendChild(recElement);
            
            // Add event listeners for recommendation actions
            this.setupRecommendationActions(recElement, rec);
        });
    }
    
    setupRecommendationActions(element, recommendation) {
        element.querySelectorAll('[data-action]').forEach(button => {
            button.addEventListener('click', (e) => {
                e.stopPropagation();
                const action = button.getAttribute('data-action');
                this.recordInteraction(recommendation.item_id, action, recommendation);
            });
        });
        
        // Click tracking for the entire item
        element.addEventListener('click', () => {
            this.recordInteraction(recommendation.item_id, 'click', recommendation);
        });
    }
    
    displayRecentInteractions(interactions) {
        const container = document.getElementById('recent-interactions');
        if (!container) return;
        
        container.innerHTML = '';
        
        interactions.slice(0, 5).forEach(interaction => {
            const interactionElement = document.createElement('div');
            interactionElement.className = 'interaction-item';
            
            const iconClass = interaction.type === 'like' ? 'bi-heart-fill' :
                            interaction.type === 'click' ? 'bi-cursor-fill' : 'bi-eye-fill';
            
            interactionElement.innerHTML = `
                <div class="interaction-icon ${interaction.type}">
                    <i class="bi ${iconClass}"></i>
                </div>
                <div class="interaction-details">
                    <div class="interaction-title">${interaction.item_title || 'Unknown Item'}</div>
                    <div class="interaction-time">${this.formatTimeAgo(interaction.timestamp)}</div>
                </div>
            `;
            
            container.appendChild(interactionElement);
        });
    }
    
    // Chart Setup
    setupCharts() {
        this.setupPerformanceChart();
        this.setupAlgorithmChart();
    }
    
    setupPerformanceChart() {
        const ctx = document.getElementById('performance-chart');
        if (!ctx) return;
        
        this.charts.performance = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Click-Through Rate',
                    data: [],
                    borderColor: '#0d6efd',
                    backgroundColor: 'rgba(13, 110, 253, 0.1)',
                    tension: 0.4
                }, {
                    label: 'Conversion Rate',
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
                        max: 100,
                        ticks: {
                            callback: function(value) {
                                return value + '%';
                            }
                        }
                    }
                },
                plugins: {
                    legend: {
                        position: 'top'
                    }
                }
            }
        });
    }
    
    setupAlgorithmChart() {
        const ctx = document.getElementById('algorithm-chart');
        if (!ctx) return;
        
        this.charts.algorithm = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: ['Semantic Search', 'Collaborative Filtering', 'PageRank'],
                datasets: [{
                    data: [40, 30, 30],
                    backgroundColor: ['#0d6efd', '#198754', '#ffc107'],
                    borderWidth: 2,
                    borderColor: '#fff'
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'bottom'
                    }
                }
            }
        });
    }
    
    updatePerformanceCharts(data) {
        if (this.charts.performance && data.timeline) {
            this.charts.performance.data.labels = data.timeline.labels;
            this.charts.performance.data.datasets[0].data = data.timeline.ctr;
            this.charts.performance.data.datasets[1].data = data.timeline.conversion_rate;
            this.charts.performance.update();
        }
        
        if (this.charts.algorithm && data.algorithm_performance) {
            const algorithms = Object.keys(data.algorithm_performance);
            const performance = Object.values(data.algorithm_performance);
            
            this.charts.algorithm.data.labels = algorithms;
            this.charts.algorithm.data.datasets[0].data = performance;
            this.charts.algorithm.update();
        }
    }
    
    // User Interactions
    async recordInteraction(itemId, type, recommendation) {
        try {
            const interaction = {
                user_id: this.currentUser,
                item_id: itemId,
                interaction_type: type,
                timestamp: new Date().toISOString(),
                session_id: this.getSessionId(),
                context: {
                    source: 'dashboard',
                    recommendation_id: recommendation.recommendation_id,
                    position: recommendation.position
                }
            };
            
            const response = await this.fetchWithAuth('/api/v1/interactions/explicit', {
                method: 'POST',
                body: JSON.stringify(interaction)
            });
            
            if (response.ok) {
                this.showNotification(`${type.charAt(0).toUpperCase() + type.slice(1)} recorded`, 'success');
                
                // Update UI based on interaction
                this.updateInteractionUI(itemId, type);
            }
        } catch (error) {
            console.error('Error recording interaction:', error);
            this.showNotification('Error recording interaction', 'error');
        }
    }
    
    updateInteractionUI(itemId, type) {
        const itemElement = document.querySelector(`[data-item-id="${itemId}"]`);
        if (!itemElement) return;
        
        // Add visual feedback
        itemElement.classList.add('interaction-feedback');
        setTimeout(() => itemElement.classList.remove('interaction-feedback'), 1000);
        
        // Update button states
        if (type === 'like') {
            const likeBtn = itemElement.querySelector('.like-btn');
            likeBtn.classList.add('btn-success');
            likeBtn.classList.remove('btn-outline-primary');
        } else if (type === 'dislike') {
            const dislikeBtn = itemElement.querySelector('.dislike-btn');
            dislikeBtn.classList.add('btn-danger');
            dislikeBtn.classList.remove('btn-outline-secondary');
        }
    }
    
    // Utility Functions
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
    
    generateStarRating(rating) {
        const fullStars = Math.floor(rating);
        const hasHalfStar = rating % 1 >= 0.5;
        let stars = '';
        
        for (let i = 0; i < fullStars; i++) {
            stars += '<i class="bi bi-star-fill"></i>';
        }
        
        if (hasHalfStar) {
            stars += '<i class="bi bi-star-half"></i>';
        }
        
        const emptyStars = 5 - fullStars - (hasHalfStar ? 1 : 0);
        for (let i = 0; i < emptyStars; i++) {
            stars += '<i class="bi bi-star"></i>';
        }
        
        return stars;
    }
    
    getSessionId() {
        let sessionId = sessionStorage.getItem('sessionId');
        if (!sessionId) {
            sessionId = 'session-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
            sessionStorage.setItem('sessionId', sessionId);
        }
        return sessionId;
    }
    
    setLoadingState(elementId, loading) {
        const element = document.getElementById(elementId);
        if (!element) return;
        
        if (loading) {
            element.classList.add('loading');
            element.innerHTML = `
                <div class="text-center p-4">
                    <div class="spinner-border" role="status">
                        <span class="visually-hidden">Loading...</span>
                    </div>
                </div>
            `;
        } else {
            element.classList.remove('loading');
        }
    }
    
    showNotification(message, type = 'info') {
        const toast = document.getElementById('notification-toast');
        const messageElement = document.getElementById('toast-message');
        
        if (!toast || !messageElement) return;
        
        messageElement.textContent = message;
        
        // Update toast styling based on type
        toast.className = `toast ${type === 'error' ? 'bg-danger text-white' : 
                                  type === 'success' ? 'bg-success text-white' : 
                                  type === 'warning' ? 'bg-warning text-dark' : ''}`;
        
        const bsToast = new bootstrap.Toast(toast);
        bsToast.show();
    }
    
    debounce(func, wait) {
        clearTimeout(this.debounceTimer);
        this.debounceTimer = setTimeout(func, wait);
    }
    
    // Periodic Updates
    startPeriodicUpdates() {
        if (!this.config) return;
        
        // Update metrics based on config
        setInterval(() => {
            if (this.currentSection === 'dashboard') {
                this.loadDashboardData();
            }
        }, this.config.ui.refreshIntervals.metrics);
        
        // Update health status based on config
        setInterval(() => {
            this.loadHealthStatus();
        }, this.config.ui.refreshIntervals.health);
        
        // Send WebSocket ping based on config
        if (this.config.features.websocket) {
            setInterval(() => {
                if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                    this.ws.send(JSON.stringify({ type: 'ping' }));
                }
            }, this.config.ui.refreshIntervals.websocketPing);
        }
    }
    
    // Event Handlers for WebSocket messages
    handleRecommendationUpdate(data) {
        if (this.currentSection === 'recommendations') {
            this.loadRecommendations();
        }
        this.showNotification('New recommendations available!', 'info');
    }
    
    handleMetricsUpdate(data) {
        if (this.currentSection === 'dashboard') {
            this.updateMetricsDisplay(data.metrics);
        }
    }
    
    handleSystemStatus(data) {
        // Update connection count or other system info if needed
        console.log('System status update:', data);
    }
    
    // Additional methods for filters, preferences, etc.
    toggleFilters() {
        const filtersPanel = document.getElementById('filters-panel');
        if (filtersPanel) {
            filtersPanel.style.display = filtersPanel.style.display === 'none' ? 'block' : 'none';
        }
    }
    
    applyFilters() {
        // Implementation for applying filters
        this.loadRecommendations();
    }
    
    loadMoreRecommendations() {
        // Implementation for loading more recommendations
        console.log('Loading more recommendations...');
    }
    
    savePreferences() {
        // Implementation for saving user preferences
        this.showNotification('Preferences saved successfully!', 'success');
    }
    
    viewRecommendationHistory() {
        // Implementation for viewing recommendation history
        console.log('Viewing recommendation history...');
    }
    
    searchRecommendations(query) {
        // Implementation for searching recommendations
        console.log('Searching for:', query);
    }
}

// Initialize dashboard when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new RecommendationDashboard();
});