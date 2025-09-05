require('dotenv').config();
console.log('Environment variables loaded:');
console.log('API_BASE_URL:', process.env.API_BASE_URL);
console.log('API_KEY:', process.env.API_KEY);
console.log('DEFAULT_USER_ID:', process.env.DEFAULT_USER_ID);

const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const path = require('path');
const cors = require('cors');
const helmet = require('helmet');
const compression = require('compression');
const WebSocket = require('ws');
const http = require('http');

const app = express();
const server = http.createServer(app);
const wss = new WebSocket.Server({ server });

const PORT = process.env.PORT || 3000;
const API_URL = process.env.API_URL || 'http://localhost:8080';

// Security middleware
app.use(helmet({
    contentSecurityPolicy: {
        directives: {
            defaultSrc: ["'self'"],
            styleSrc: ["'self'", "'unsafe-inline'", "https://cdn.jsdelivr.net"],
            scriptSrc: ["'self'", "'unsafe-inline'", "https://cdn.jsdelivr.net"],
            imgSrc: ["'self'", "data:", "https:"],
            connectSrc: ["'self'", API_URL, `ws://localhost:${PORT}`],
        },
    },
}));

// CORS and compression
app.use(cors());
app.use(compression());

// Parse JSON bodies
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true, limit: '10mb' }));

// Serve static files
app.use(express.static(path.join(__dirname, 'public')));

// API proxy to Go backend
app.use('/api', createProxyMiddleware({
    target: API_URL,
    changeOrigin: true,
    timeout: 30000,
    proxyTimeout: 30000,
    onError: (err, req, res) => {
        console.error('Proxy error:', err);
        res.status(500).json({
            error: 'Backend service unavailable',
            message: 'Please try again later'
        });
    },
    onProxyReq: (proxyReq, req, res) => {
        console.log(`Proxying ${req.method} ${req.url} to ${API_URL}${req.url}`);
    }
}));

// Health check endpoint
app.get('/health', (req, res) => {
    res.json({
        status: 'healthy',
        timestamp: new Date().toISOString(),
        service: 'recommendation-frontend',
        uptime: process.uptime()
    });
});

// Configuration endpoint
app.get('/config/app-config.json', (req, res) => {
    const config = {
        api: {
            baseUrl: process.env.API_BASE_URL || `http://localhost:8080`,
            version: process.env.API_VERSION || "v1",
            timeout: parseInt(process.env.API_TIMEOUT) || 30000
        },
        auth: {
            apiKey: process.env.API_KEY || "demo-premium-key",
            defaultUserId: process.env.DEFAULT_USER_ID || "550e8400-e29b-41d4-a716-446655440000"
        },
        features: {
            websocket: (process.env.ENABLE_WEBSOCKET || 'true') === 'true',
            realTimeUpdates: (process.env.ENABLE_REAL_TIME_UPDATES || 'true') === 'true',
            notifications: (process.env.ENABLE_NOTIFICATIONS || 'true') === 'true'
        },
        ui: {
            theme: process.env.UI_THEME || "light",
            refreshIntervals: {
                metrics: parseInt(process.env.METRICS_REFRESH_INTERVAL) || 300000,
                health: parseInt(process.env.HEALTH_REFRESH_INTERVAL) || 30000,
                websocketPing: parseInt(process.env.WEBSOCKET_PING_INTERVAL) || 60000
            }
        },
        development: {
            enableDebugLogs: (process.env.ENABLE_DEBUG_LOGS || 'true') === 'true',
            mockData: (process.env.ENABLE_MOCK_DATA || 'false') === 'true'
        }
    };

    res.json(config);
});

// Debug endpoint to check environment variables
app.get('/debug/env', (req, res) => {
    res.json({
        API_BASE_URL: process.env.API_BASE_URL,
        API_KEY: process.env.API_KEY,
        DEFAULT_USER_ID: process.env.DEFAULT_USER_ID,
        ENABLE_DEBUG_LOGS: process.env.ENABLE_DEBUG_LOGS
    });
});

// WebSocket connection for real-time updates
wss.on('connection', (ws, req) => {
    console.log('New WebSocket connection established');

    // Send welcome message
    ws.send(JSON.stringify({
        type: 'connection',
        message: 'Connected to recommendation engine',
        timestamp: new Date().toISOString()
    }));

    // Handle incoming messages
    ws.on('message', (message) => {
        try {
            const data = JSON.parse(message);
            console.log('Received WebSocket message:', data);

            // Handle different message types
            switch (data.type) {
                case 'subscribe':
                    // Subscribe to user-specific updates
                    ws.userId = data.userId;
                    ws.send(JSON.stringify({
                        type: 'subscribed',
                        userId: data.userId,
                        timestamp: new Date().toISOString()
                    }));
                    break;

                case 'ping':
                    ws.send(JSON.stringify({
                        type: 'pong',
                        timestamp: new Date().toISOString()
                    }));
                    break;

                default:
                    console.log('Unknown message type:', data.type);
            }
        } catch (error) {
            console.error('Error parsing WebSocket message:', error);
            ws.send(JSON.stringify({
                type: 'error',
                message: 'Invalid message format',
                timestamp: new Date().toISOString()
            }));
        }
    });

    // Handle connection close
    ws.on('close', () => {
        console.log('WebSocket connection closed');
    });

    // Handle errors
    ws.on('error', (error) => {
        console.error('WebSocket error:', error);
    });
});

// Broadcast function for real-time updates
function broadcastToUser(userId, data) {
    wss.clients.forEach((client) => {
        if (client.readyState === WebSocket.OPEN && client.userId === userId) {
            client.send(JSON.stringify({
                ...data,
                timestamp: new Date().toISOString()
            }));
        }
    });
}

// Broadcast function for all clients
function broadcastToAll(data) {
    wss.clients.forEach((client) => {
        if (client.readyState === WebSocket.OPEN) {
            client.send(JSON.stringify({
                ...data,
                timestamp: new Date().toISOString()
            }));
        }
    });
}

// Periodic system status broadcast
setInterval(() => {
    broadcastToAll({
        type: 'system_status',
        status: 'healthy',
        connections: wss.clients.size,
        uptime: process.uptime()
    });
}, 30000); // Every 30 seconds

// Routes
app.get('/', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'index.html'));
});

app.get('/admin', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'admin.html'));
});

app.get('/dashboard', (req, res) => {
    res.sendFile(path.join(__dirname, 'public', 'dashboard.html'));
});

// Error handling middleware
app.use((err, req, res, next) => {
    console.error('Express error:', err);
    res.status(500).json({
        error: 'Internal server error',
        message: process.env.NODE_ENV === 'development' ? err.message : 'Something went wrong'
    });
});

// 404 handler
app.use((req, res) => {
    res.status(404).json({
        error: 'Not found',
        message: `Route ${req.method} ${req.url} not found`
    });
});

// Graceful shutdown
process.on('SIGTERM', () => {
    console.log('SIGTERM received, shutting down gracefully');
    server.close(() => {
        console.log('Server closed');
        process.exit(0);
    });
});

process.on('SIGINT', () => {
    console.log('SIGINT received, shutting down gracefully');
    server.close(() => {
        console.log('Server closed');
        process.exit(0);
    });
});

// Start server
server.listen(PORT, () => {
    console.log(`Frontend server running on port ${PORT}`);
    console.log(`Proxying API requests to ${API_URL}`);
    console.log(`WebSocket server ready for connections`);
});

// Export for testing
module.exports = { app, server, broadcastToUser, broadcastToAll };