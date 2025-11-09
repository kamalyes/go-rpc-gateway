# Go RPC Gateway - 性能测试脚本
# k6 Load Testing Scripts

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// 自定义指标
const errorRate = new Rate('error_rate');
const responseTime = new Trend('response_time');
const requestCount = new Counter('request_count');

// 测试配置
export const options = {
  // 负载测试阶段
  stages: [
    { duration: '2m', target: 100 },   // 2分钟内逐步增加到100用户
    { duration: '5m', target: 100 },   // 维持100用户5分钟
    { duration: '2m', target: 200 },   // 2分钟内增加到200用户  
    { duration: '5m', target: 200 },   // 维持200用户5分钟
    { duration: '2m', target: 500 },   // 2分钟内增加到500用户
    { duration: '10m', target: 500 },  // 维持500用户10分钟
    { duration: '3m', target: 1000 },  // 3分钟内增加到1000用户
    { duration: '5m', target: 1000 },  // 维持1000用户5分钟
    { duration: '5m', target: 0 },     // 5分钟内减少到0用户
  ],
  
  // 性能阈值
  thresholds: {
    'http_req_duration': ['p(95)<500', 'p(99)<1000'], // 95%请求<500ms，99%请求<1s
    'http_req_failed': ['rate<0.01'],                  // 错误率<1%
    'error_rate': ['rate<0.02'],                       // 自定义错误率<2%
    'response_time': ['p(95)<400'],                    // 95%响应时间<400ms
  },
};

// 测试数据
const testData = {
  users: [
    { username: 'user1', password: 'password123' },
    { username: 'user2', password: 'password456' },
    { username: 'user3', password: 'password789' },
  ],
  apiKeys: [
    'test-key-1',
    'test-key-2', 
    'test-key-3',
  ],
  endpoints: [
    '/api/users',
    '/api/orders',
    '/api/products',
    '/api/analytics',
  ]
};

// 基础URL配置
const baseUrl = __ENV.BASE_URL || 'http://localhost:8080';
const grpcUrl = __ENV.GRPC_URL || 'http://localhost:9090';

// 公共请求参数
const headers = {
  'Content-Type': 'application/json',
  'Accept': 'application/json',
  'User-Agent': 'k6-load-test/1.0',
};

// 主测试函数
export default function() {
  const testScenario = Math.random();
  
  if (testScenario < 0.3) {
    // 30% - 认证测试
    authenticationTest();
  } else if (testScenario < 0.6) {
    // 30% - API访问测试
    apiAccessTest();
  } else if (testScenario < 0.8) {
    // 20% - 混合负载测试
    mixedLoadTest();
  } else {
    // 20% - 性能监控测试
    monitoringTest();
  }
  
  // 随机等待时间模拟真实用户行为
  sleep(Math.random() * 3 + 1); // 1-4秒随机等待
}

// 认证测试
function authenticationTest() {
  const user = testData.users[Math.floor(Math.random() * testData.users.length)];
  
  // 1. 健康检查
  const healthRes = http.get(`${baseUrl}/health`, { headers });
  check(healthRes, {
    'health check status is 200': (r) => r.status === 200,
    'health check response time < 100ms': (r) => r.timings.duration < 100,
  });
  
  // 2. 用户登录
  const loginPayload = JSON.stringify({
    username: user.username,
    password: user.password,
  });
  
  const loginRes = http.post(`${baseUrl}/api/auth/login`, loginPayload, { 
    headers: headers 
  });
  
  const loginSuccess = check(loginRes, {
    'login status is 200': (r) => r.status === 200,
    'login response has token': (r) => r.json('token') !== undefined,
    'login response time < 300ms': (r) => r.timings.duration < 300,
  });
  
  if (loginSuccess && loginRes.json('token')) {
    const token = loginRes.json('token');
    const authHeaders = {
      ...headers,
      'Authorization': `Bearer ${token}`,
    };
    
    // 3. 获取用户信息
    const profileRes = http.get(`${baseUrl}/api/users/profile`, { 
      headers: authHeaders 
    });
    
    check(profileRes, {
      'profile status is 200': (r) => r.status === 200,
      'profile has user data': (r) => r.json('user') !== undefined,
    });
    
    responseTime.add(profileRes.timings.duration);
  }
  
  requestCount.add(1);
  errorRate.add(loginRes.status !== 200);
}

// API访问测试
function apiAccessTest() {
  const apiKey = testData.apiKeys[Math.floor(Math.random() * testData.apiKeys.length)];
  const endpoint = testData.endpoints[Math.floor(Math.random() * testData.endpoints.length)];
  
  const apiHeaders = {
    ...headers,
    'X-API-Key': apiKey,
  };
  
  // 1. API调用
  const apiRes = http.get(`${baseUrl}${endpoint}`, { headers: apiHeaders });
  
  const apiSuccess = check(apiRes, {
    'api status is 200': (r) => r.status === 200,
    'api response time < 500ms': (r) => r.timings.duration < 500,
    'api response has data': (r) => r.body.length > 0,
  });
  
  // 2. 模拟数据操作
  if (apiSuccess && Math.random() < 0.3) { // 30%概率进行POST操作
    const postData = JSON.stringify({
      test: true,
      timestamp: new Date().toISOString(),
      data: `load-test-${Math.random().toString(36).substring(7)}`,
    });
    
    const postRes = http.post(`${baseUrl}${endpoint}`, postData, { 
      headers: apiHeaders 
    });
    
    check(postRes, {
      'post status is 201 or 200': (r) => r.status === 201 || r.status === 200,
      'post response time < 800ms': (r) => r.timings.duration < 800,
    });
    
    responseTime.add(postRes.timings.duration);
  }
  
  requestCount.add(1);
  errorRate.add(apiRes.status !== 200);
  responseTime.add(apiRes.timings.duration);
}

// 混合负载测试
function mixedLoadTest() {
  const batch = http.batch([
    ['GET', `${baseUrl}/health`, null, { headers }],
    ['GET', `${baseUrl}/ready`, null, { headers }],
    ['GET', `${baseUrl}/metrics`, null, { headers }],
    ['GET', `${baseUrl}/api/version`, null, { headers }],
  ]);
  
  batch.forEach((res, index) => {
    check(res, {
      [`batch request ${index} status is 200`]: (r) => r.status === 200,
      [`batch request ${index} response time < 200ms`]: (r) => r.timings.duration < 200,
    });
    
    responseTime.add(res.timings.duration);
    errorRate.add(res.status !== 200);
  });
  
  requestCount.add(batch.length);
}

// 监控测试
function monitoringTest() {
  // 1. Prometheus指标
  const metricsRes = http.get(`${baseUrl}/metrics`, { headers });
  check(metricsRes, {
    'metrics endpoint available': (r) => r.status === 200,
    'metrics contain gateway data': (r) => r.body.includes('gateway_'),
    'metrics response time < 150ms': (r) => r.timings.duration < 150,
  });
  
  // 2. PProf端点（如果启用）
  if (Math.random() < 0.1) { // 10%概率检查pprof
    const pprofHeaders = {
      ...headers,
      'Authorization': `Bearer ${__ENV.PPROF_TOKEN || 'test-token'}`,
    };
    
    const pprofRes = http.get(`${baseUrl}/debug/pprof/`, { headers: pprofHeaders });
    check(pprofRes, {
      'pprof endpoint accessible': (r) => r.status === 200 || r.status === 401,
    });
  }
  
  // 3. gRPC健康检查
  const grpcHealthRes = http.get(`${grpcUrl}/grpc/health`, { headers });
  check(grpcHealthRes, {
    'grpc health status is 200': (r) => r.status === 200,
    'grpc response time < 100ms': (r) => r.timings.duration < 100,
  });
  
  requestCount.add(1);
  responseTime.add(metricsRes.timings.duration);
}

// 测试阶段钩子函数
export function setup() {
  console.log('🚀 Starting load test...');
  console.log(`Base URL: ${baseUrl}`);
  console.log(`gRPC URL: ${grpcUrl}`);
  
  // 预热请求
  const warmupRes = http.get(`${baseUrl}/health`);
  if (warmupRes.status !== 200) {
    throw new Error(`Warmup request failed: ${warmupRes.status}`);
  }
  
  return { 
    startTime: new Date().toISOString(),
    baseUrl: baseUrl,
  };
}

export function teardown(data) {
  console.log('🏁 Load test completed');
  console.log(`Started at: ${data.startTime}`);
  console.log(`Completed at: ${new Date().toISOString()}`);
}

// 自定义摘要报告
export function handleSummary(data) {
  return {
    'loadtest-summary.json': JSON.stringify(data, null, 2),
    'loadtest-report.html': htmlReport(data),
  };
}

// 生成HTML报告
function htmlReport(data) {
  const totalRequests = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;
  const avgResponseTime = data.metrics.http_req_duration ? data.metrics.http_req_duration.values.avg : 0;
  const p95ResponseTime = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(95)'] : 0;
  const errorRate = data.metrics.http_req_failed ? data.metrics.http_req_failed.values.rate : 0;
  
  return `
<!DOCTYPE html>
<html>
<head>
    <title>Go RPC Gateway - Load Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { background: #f4f4f4; padding: 20px; border-radius: 5px; }
        .metric { margin: 20px 0; padding: 15px; border-left: 4px solid #007cba; }
        .success { border-left-color: #28a745; }
        .warning { border-left-color: #ffc107; }
        .error { border-left-color: #dc3545; }
        .chart { margin: 20px 0; height: 300px; background: #f9f9f9; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 Go RPC Gateway Load Test Report</h1>
        <p>Generated at: ${new Date().toISOString()}</p>
        <p>Base URL: ${baseUrl}</p>
    </div>
    
    <div class="metric ${errorRate < 0.01 ? 'success' : errorRate < 0.05 ? 'warning' : 'error'}">
        <h3>📊 Key Metrics</h3>
        <ul>
            <li>Total Requests: ${totalRequests}</li>
            <li>Average Response Time: ${avgResponseTime.toFixed(2)}ms</li>
            <li>95th Percentile: ${p95ResponseTime.toFixed(2)}ms</li>
            <li>Error Rate: ${(errorRate * 100).toFixed(2)}%</li>
        </ul>
    </div>
    
    <div class="metric">
        <h3>🎯 Test Summary</h3>
        <p>This load test simulated real-world usage patterns including:</p>
        <ul>
            <li>User authentication flows (30%)</li>
            <li>API access patterns (30%)</li>
            <li>Mixed endpoint usage (20%)</li>
            <li>Monitoring and health checks (20%)</li>
        </ul>
    </div>
    
    <div class="metric ${p95ResponseTime < 500 ? 'success' : p95ResponseTime < 1000 ? 'warning' : 'error'}">
        <h3>⚡ Performance Analysis</h3>
        <p><strong>Response Time Target:</strong> 95% of requests under 500ms</p>
        <p><strong>Actual 95th Percentile:</strong> ${p95ResponseTime.toFixed(2)}ms</p>
        <p><strong>Status:</strong> ${p95ResponseTime < 500 ? '✅ PASSED' : p95ResponseTime < 1000 ? '⚠️ WARNING' : '❌ FAILED'}</p>
    </div>
    
    <div class="metric ${errorRate < 0.01 ? 'success' : 'error'}">
        <h3>🛡️ Reliability Analysis</h3>
        <p><strong>Error Rate Target:</strong> Less than 1%</p>
        <p><strong>Actual Error Rate:</strong> ${(errorRate * 100).toFixed(2)}%</p>
        <p><strong>Status:</strong> ${errorRate < 0.01 ? '✅ PASSED' : '❌ FAILED'}</p>
    </div>
</body>
</html>
  `;
}