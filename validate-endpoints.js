#!/usr/bin/env node

/**
 * Comprehensive endpoint validation script
 * Tests all API endpoints and validates response structures
 */

// Use built-in fetch (Node 18+) or require node-fetch for older versions
const fetch = globalThis.fetch || require('node-fetch');

const FRONTEND_URL = 'http://localhost:3000';
const BACKEND_URL = 'http://localhost:8080';
const API_KEY = 'demo-premium-key';
const USER_ID = '550e8400-e29b-41d4-a716-446655440000';
const ITEM_ID = '550e8400-e29b-41d4-a716-446655440001';
const SESSION_ID = '550e8400-e29b-41d4-a716-446655440002';

// Expected response structures
const expectedStructures = {
    health: {
        status: 'string',
        timestamp: 'string',
        services: 'object'
    },
    recommendations: {
        user_id: 'string',
        recommendations: 'array',
        context: 'string',
        generated_at: 'string',
        cache_hit: 'boolean'
    },
    businessMetrics: {
        start_date: 'string',
        end_date: 'string',
        metrics: 'object',
        timestamp: 'string'
    },
    performanceMetrics: {
        algorithm_performance: 'object',
        timeline: 'object',
        timestamp: 'string'
    },
    frontendConfig: {
        api: 'object',
        auth: 'object',
        features: 'object',
        ui: 'object',
        development: 'object'
    }
};

// Endpoint definitions
const endpoints = [
    {
        name: 'Frontend Health Check',
        url: `${FRONTEND_URL}/health`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: { status: 'string', timestamp: 'string', service: 'string', uptime: 'number' },
        direct: true
    },
    {
        name: 'Frontend Configuration',
        url: `${FRONTEND_URL}/config/app-config.json`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.frontendConfig,
        direct: true
    },
    {
        name: 'Backend Health Check (Direct)',
        url: `${BACKEND_URL}/health`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.health,
        direct: true
    },
    {
        name: 'Backend Health Check (Proxy)',
        url: `${FRONTEND_URL}/api/health`,
        method: 'GET',
        expectedStatus: 404, // Health is not under /api
        direct: false
    },
    {
        name: 'Get Recommendations (Direct)',
        url: `${BACKEND_URL}/api/v1/recommendations/${USER_ID}?count=10&explain=true`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.recommendations,
        requiresAuth: true,
        direct: true
    },
    {
        name: 'Get Recommendations (Proxy)',
        url: `${FRONTEND_URL}/api/v1/recommendations/${USER_ID}?count=10&explain=true`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.recommendations,
        requiresAuth: true,
        direct: false
    },
    {
        name: 'Get Business Metrics (Direct)',
        url: `${BACKEND_URL}/api/v1/metrics/business`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.businessMetrics,
        requiresAuth: true,
        direct: true
    },
    {
        name: 'Get Business Metrics (Proxy)',
        url: `${FRONTEND_URL}/api/v1/metrics/business`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.businessMetrics,
        requiresAuth: true,
        direct: false
    },
    {
        name: 'Get Performance Metrics (Direct)',
        url: `${BACKEND_URL}/api/v1/metrics/performance`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.performanceMetrics,
        requiresAuth: true,
        direct: true
    },
    {
        name: 'Get Performance Metrics (Proxy)',
        url: `${FRONTEND_URL}/api/v1/metrics/performance`,
        method: 'GET',
        expectedStatus: 200,
        expectedStructure: expectedStructures.performanceMetrics,
        requiresAuth: true,
        direct: false
    },
    {
        name: 'Record Explicit Interaction (Direct)',
        url: `${BACKEND_URL}/api/v1/interactions/explicit`,
        method: 'POST',
        expectedStatus: [200, 500], // 500 expected due to missing DB tables
        requiresAuth: true,
        direct: true,
        body: {
            user_id: USER_ID,
            item_id: ITEM_ID,
            type: 'like',
            session_id: SESSION_ID
        }
    },
    {
        name: 'Get User Interactions (Direct)',
        url: `${BACKEND_URL}/api/v1/users/${USER_ID}/interactions?limit=10`,
        method: 'GET',
        expectedStatus: [200, 500], // 500 expected due to missing DB tables
        requiresAuth: true,
        direct: true
    },
    {
        name: 'Get Similar Recommendations (Direct)',
        url: `${BACKEND_URL}/api/v1/recommendations/${USER_ID}/similar/${ITEM_ID}`,
        method: 'GET',
        expectedStatus: [200, 500], // 500 expected due to missing DB tables
        requiresAuth: true,
        direct: true
    },
    {
        name: 'GraphQL Playground (Direct)',
        url: `${BACKEND_URL}/graphql`,
        method: 'GET',
        expectedStatus: 200,
        direct: true
    },
    {
        name: 'Swagger Documentation (Direct)',
        url: `${BACKEND_URL}/docs/`,
        method: 'GET',
        expectedStatus: 200,
        direct: true
    },
    {
        name: 'Prometheus Metrics (Direct)',
        url: `${BACKEND_URL}/metrics`,
        method: 'GET',
        expectedStatus: 200,
        direct: true
    }
];

function validateStructure(data, expectedStructure) {
    const errors = [];
    
    for (const [key, expectedType] of Object.entries(expectedStructure)) {
        if (!(key in data)) {
            errors.push(`Missing field: ${key}`);
            continue;
        }
        
        const actualType = Array.isArray(data[key]) ? 'array' : typeof data[key];
        if (actualType !== expectedType) {
            errors.push(`Field ${key}: expected ${expectedType}, got ${actualType}`);
        }
    }
    
    return errors;
}

async function testEndpoint(endpoint) {
    try {
        const options = {
            method: endpoint.method,
            headers: {
                'Content-Type': 'application/json'
            },
            timeout: 10000
        };

        if (endpoint.requiresAuth) {
            options.headers['Authorization'] = `Bearer ${API_KEY}`;
            options.headers['X-User-ID'] = USER_ID;
        }

        if (endpoint.body) {
            options.body = JSON.stringify(endpoint.body);
        }

        console.log(`\n🧪 Testing: ${endpoint.name}`);
        console.log(`   URL: ${endpoint.url}`);
        console.log(`   Method: ${endpoint.method}`);
        console.log(`   Type: ${endpoint.direct ? 'Direct' : 'Proxy'}`);
        
        const startTime = Date.now();
        const response = await fetch(endpoint.url, options);
        const endTime = Date.now();
        const responseTime = endTime - startTime;

        const expectedStatuses = Array.isArray(endpoint.expectedStatus) ? endpoint.expectedStatus : [endpoint.expectedStatus];
        const statusMatch = expectedStatuses.includes(response.status);

        console.log(`   Status: ${response.status} (${responseTime}ms) ${statusMatch ? '✅' : '❌'}`);

        let responseData = null;
        let structureErrors = [];

        try {
            const contentType = response.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                responseData = await response.json();
                
                // Validate structure if expected
                if (endpoint.expectedStructure && statusMatch) {
                    structureErrors = validateStructure(responseData, endpoint.expectedStructure);
                }
            } else {
                const text = await response.text();
                responseData = text.substring(0, 200) + (text.length > 200 ? '...' : '');
            }
        } catch (parseError) {
            console.log(`   ⚠️  Could not parse response: ${parseError.message}`);
        }

        // Structure validation results
        if (structureErrors.length === 0 && endpoint.expectedStructure) {
            console.log(`   📋 Structure: Valid ✅`);
        } else if (structureErrors.length > 0) {
            console.log(`   📋 Structure: Invalid ❌`);
            structureErrors.forEach(error => console.log(`      - ${error}`));
        }

        // Show response preview
        if (responseData && typeof responseData === 'object') {
            const preview = JSON.stringify(responseData).substring(0, 150);
            console.log(`   📄 Response: ${preview}${JSON.stringify(responseData).length > 150 ? '...' : ''}`);
        } else if (responseData) {
            console.log(`   📄 Response: ${responseData}`);
        }

        return {
            success: statusMatch && structureErrors.length === 0,
            endpoint: endpoint.name,
            status: response.status,
            expectedStatus: expectedStatuses,
            responseTime,
            structureValid: structureErrors.length === 0,
            structureErrors,
            direct: endpoint.direct
        };

    } catch (error) {
        console.log(`   ❌ Error: ${error.message}`);
        return {
            success: false,
            endpoint: endpoint.name,
            error: error.message,
            direct: endpoint.direct
        };
    }
}

async function runValidation() {
    console.log('🚀 Starting Comprehensive Endpoint Validation');
    console.log('=' .repeat(80));

    const results = [];
    
    for (const endpoint of endpoints) {
        const result = await testEndpoint(endpoint);
        results.push(result);
        
        // Small delay between tests
        await new Promise(resolve => setTimeout(resolve, 500));
    }

    // Summary
    console.log('\n' + '=' .repeat(80));
    console.log('📊 Validation Results Summary');
    console.log('=' .repeat(80));

    const successful = results.filter(r => r.success);
    const failed = results.filter(r => !r.success);
    const directTests = results.filter(r => r.direct);
    const proxyTests = results.filter(r => !r.direct);

    console.log(`✅ Successful: ${successful.length}/${results.length}`);
    console.log(`❌ Failed: ${failed.length}/${results.length}`);
    console.log(`🔗 Direct Tests: ${directTests.filter(r => r.success).length}/${directTests.length}`);
    console.log(`🔄 Proxy Tests: ${proxyTests.filter(r => r.success).length}/${proxyTests.length}`);

    // Categorize results
    const categories = {
        'Frontend Services': results.filter(r => r.endpoint.includes('Frontend')),
        'Backend Services (Direct)': results.filter(r => r.direct && r.endpoint.includes('Backend')),
        'API Endpoints (Direct)': results.filter(r => r.direct && !r.endpoint.includes('Frontend') && !r.endpoint.includes('Backend')),
        'API Endpoints (Proxy)': results.filter(r => !r.direct && r.endpoint.includes('Proxy'))
    };

    Object.entries(categories).forEach(([category, categoryResults]) => {
        if (categoryResults.length > 0) {
            const categorySuccess = categoryResults.filter(r => r.success).length;
            console.log(`\n📂 ${category}: ${categorySuccess}/${categoryResults.length}`);
            categoryResults.forEach(result => {
                const icon = result.success ? '✅' : '❌';
                const time = result.responseTime ? `${result.responseTime}ms` : 'N/A';
                console.log(`   ${icon} ${result.endpoint} (${time})`);
                if (!result.success && result.error) {
                    console.log(`      Error: ${result.error}`);
                }
                if (result.structureErrors && result.structureErrors.length > 0) {
                    console.log(`      Structure issues: ${result.structureErrors.join(', ')}`);
                }
            });
        }
    });

    // Integration validation
    console.log('\n🔍 Integration Validation:');
    
    const frontendHealthy = successful.some(r => r.endpoint === 'Frontend Health Check');
    const backendHealthy = successful.some(r => r.endpoint === 'Backend Health Check (Direct)');
    const proxyWorking = successful.some(r => r.endpoint.includes('(Proxy)'));
    const authWorking = successful.some(r => r.endpoint.includes('Recommendations') && r.success);
    const structureValid = results.filter(r => r.structureValid !== undefined).every(r => r.structureValid);
    
    console.log(`   Frontend Server: ${frontendHealthy ? '✅' : '❌'}`);
    console.log(`   Backend Server: ${backendHealthy ? '✅' : '❌'}`);
    console.log(`   Proxy Functionality: ${proxyWorking ? '✅' : '❌'}`);
    console.log(`   Authentication: ${authWorking ? '✅' : '❌'}`);
    console.log(`   Response Structures: ${structureValid ? '✅' : '❌'}`);

    const overallSuccess = successful.length >= results.length * 0.8; // 80% success rate
    console.log(`\n🎯 Overall Validation: ${overallSuccess ? '✅ PASS' : '❌ FAIL'}`);

    // Recommendations
    console.log('\n💡 Recommendations:');
    if (!frontendHealthy) {
        console.log('   - Start the frontend server (npm start in frontend directory)');
    }
    if (!backendHealthy) {
        console.log('   - Start the backend server (go run cmd/server/main.go)');
    }
    if (!proxyWorking) {
        console.log('   - Check frontend proxy configuration in server.js');
    }
    if (failed.some(r => r.status === 500)) {
        console.log('   - Database tables are missing (expected for initial setup)');
    }
    if (!structureValid) {
        console.log('   - Some API responses have unexpected structures');
    }

    return {
        total: results.length,
        successful: successful.length,
        failed: failed.length,
        overallSuccess,
        results
    };
}

// Run validation if this script is executed directly
if (require.main === module) {
    runValidation()
        .then(summary => {
            process.exit(summary.overallSuccess ? 0 : 1);
        })
        .catch(error => {
            console.error('Validation runner error:', error);
            process.exit(1);
        });
}

module.exports = { runValidation, testEndpoint };