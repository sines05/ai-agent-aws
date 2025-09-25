// Global state
let websocket = null;
let currentTab = 'agent-tab';

// Initialize the application
document.addEventListener('DOMContentLoaded', function() {
    initializeWebSocket();
    loadState();
    
    // Initialize mermaid
    mermaid.initialize({ 
        startOnLoad: true,
        theme: 'default',
        themeVariables: {
            primaryColor: '#007bff',
            primaryTextColor: '#333',
            primaryBorderColor: '#dee2e6',
            lineColor: '#666'
        }
    });
});

// WebSocket Connection
function initializeWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    try {
        websocket = new WebSocket(wsUrl);
        
        websocket.onopen = function() {
            console.log('WebSocket connection established');
            updateConnectionStatus('Connected', 'success');
        };
        
        websocket.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                handleWebSocketMessage(data);
            } catch (error) {
                console.error('Failed to parse WebSocket message:', error);
            }
        };
        
        websocket.onclose = function(event) {
            console.log('WebSocket connection closed:', event.code, event.reason);
            updateConnectionStatus('Disconnected', 'danger');
            
            // Don't reconnect immediately if it was a normal close
            if (event.code !== 1000) {
                // Attempt to reconnect after 5 seconds for abnormal closes
                setTimeout(initializeWebSocket, 5000);
            }
        };
        
        websocket.onerror = function(error) {
            console.error('WebSocket error:', error);
            updateConnectionStatus('Connection Error', 'warning');
        };
        
        // Handle connection cleanup on page unload
        window.addEventListener('beforeunload', function() {
            if (websocket && websocket.readyState === WebSocket.OPEN) {
                websocket.close(1000, 'Page unloading');
            }
        });
        
    } catch (error) {
        console.error('WebSocket connection failed:', error);
        updateConnectionStatus('WebSocket Unavailable', 'warning');
    }
}

function handleWebSocketMessage(data) {
    if (data.type === 'state_update') {
        // Update state display if currently viewing state tab
        if (currentTab === 'state-tab') {
            displayState(data.data);
        }
    } else if (data.type === 'processing_started') {
        updateStatus(`Processing: ${data.request}`);
        showLoader();
    } else if (data.type === 'processing_completed') {
        if (data.success) {
            updateStatus('Processing completed successfully');
            // Auto-refresh state if currently viewing state tab
            if (currentTab === 'state-tab') {
                loadState(true); // Force fresh discovery
            }
        } else {
            updateStatus('Processing failed');
        }
        hideLoader();
    } else if (data.type === 'execution_started') {
        addGlobalExecutionLog('info', `Execution started for decision: ${data.decisionId}`);
        updateExecutionStatus('Initializing execution...');
    } else if (data.type === 'step_started') {
        addStepLog(data.stepId, 'info', data.message);
        addGlobalExecutionLog('info', data.message);
        updateStepStatus(data.stepId, 'running');
    } else if (data.type === 'step_progress') {
        addStepLog(data.stepId, 'info', data.message);
    } else if (data.type === 'step_completed') {
        addStepLog(data.stepId, 'success', data.message);
        addGlobalExecutionLog('success', data.message);
        updateStepStatus(data.stepId, 'completed');
    } else if (data.type === 'step_failed') {
        addStepLog(data.stepId, 'error', data.message);
        addGlobalExecutionLog('error', data.message);
        updateStepStatus(data.stepId, 'failed');
    } else if (data.type === 'step_recovery_generating') {
        // New: Handle recovery generation notification
        addStepLog(data.stepId, 'recovery-generating', data.message);
        addGlobalExecutionLog('recovery-generating', data.message);
        updateStepStatus(data.stepId, 'recovery-generating');
    } else if (data.type === 'step_recovery_needed') {
        // New: Handle recovery requests
        console.log('Recovery needed:', data);
        addStepLog(data.stepId, 'recovery', `Step failed - recovery options available`);
        addGlobalExecutionLog('recovery', `Step ${data.stepId} failed - recovery options available`);
        updateStepStatus(data.stepId, 'recovery-pending');
        
        console.log('Calling showRecoveryDialog with:', {
            failureContext: data.failureContext,
            recoveryOptions: data.recoveryOptions,
            stepId: data.stepId
        });
        
        try {
            showRecoveryDialog(data.failureContext, data.recoveryOptions, data.stepId);
        } catch (error) {
            console.error('Error showing recovery dialog:', error);
            addStepLog(data.stepId, 'error', `Failed to show recovery dialog: ${error.message}`);
        }
    } else if (data.type === 'step_recovery_started') {
        // New: Handle recovery start
        addStepLog(data.stepId, 'recovery', data.message || 'Recovery started');
        addGlobalExecutionLog('recovery', data.message || `Recovery started for step ${data.stepId}`);
        updateStepStatus(data.stepId, 'recovery-in-progress');
        // Remove any recovery progress indicators
        const progressElements = document.querySelectorAll('.recovery-progress');
        progressElements.forEach(el => el.remove());
    } else if (data.type === 'step_recovery_progress') {
        // Handle multi-step recovery progress updates
        addStepLog(data.stepId, 'recovery-progress', data.message || 'Recovery progress');
        
        // Update or create progress indicator in the recovery container
        const recoveryContainer = document.querySelector(`[data-step-id="${data.stepId}"] + .inline-recovery-container`);
        if (recoveryContainer) {
            updateMultiStepProgress(recoveryContainer, data.message, data.stepId);
        }
    } else if (data.type === 'step_recovery_completed') {
        // New: Handle recovery completion
        addStepLog(data.stepId, 'recovery-success', data.message || 'Recovery completed');
        addGlobalExecutionLog('recovery-success', data.message || `Recovery completed for step ${data.stepId}`);
        updateStepStatus(data.stepId, 'completed'); // Mark the original step as completed
        
        // Remove recovery progress UI and loading indicators
        const recoveryContainer = document.querySelector(`[data-step-id="${data.stepId}"] + .inline-recovery-container`);
        if (recoveryContainer) {
            recoveryContainer.remove();
        }
    } else if (data.type === 'step_recovery_failed') {
        // New: Handle recovery failure
        addStepLog(data.stepId, 'recovery-failed', data.message || 'Recovery failed');
        addGlobalExecutionLog('recovery-failed', data.message || `Recovery failed for step ${data.stepId}`);
        updateStepStatus(data.stepId, 'failed'); // Mark the original step as failed
        
        // Remove recovery progress UI and loading indicators
        const recoveryContainer = document.querySelector(`[data-step-id="${data.stepId}"] + .inline-recovery-container`);
        if (recoveryContainer) {
            // Show failure message in recovery container before removing it
            recoveryContainer.innerHTML = `
                <div class="recovery-step recovery-failed-final">
                    <div class="recovery-header">
                        <i class="fas fa-times-circle"></i>
                        <span class="recovery-title">Recovery Failed</span>
                    </div>
                    <div class="recovery-failure-info">
                        ${escapeHtml(data.message || 'Recovery attempts were unsuccessful')}
                    </div>
                </div>
            `;
            
            // Remove the container after a short delay to let user see the failure message
            setTimeout(() => {
                if (recoveryContainer.parentNode) {
                    recoveryContainer.remove();
                }
            }, 3000);
        }
        
        // Remove any other recovery progress indicators
        const progressElements = document.querySelectorAll('.recovery-progress, .recovery-in-progress');
        progressElements.forEach(el => el.remove());
    } else if (data.type === 'execution_completed') {
        addGlobalExecutionLog('success', data.message);
        updateExecutionStatus('Execution completed successfully');
        updateExecutionProgress(100);
        stopExecutionTimer();
        
        // Update the execution plan UI to show success state
        const executionPlan = document.getElementById('executionPlan');
        if (executionPlan) {
            executionPlan.classList.add('execution-success');
        }
        
        // Update execution header to show success
        const executionStatus = document.getElementById('executionStatus');
        if (executionStatus) {
            executionStatus.className = 'execution-status-success';
        }
        
        // Auto-refresh state after execution to show new infrastructure
        if (currentTab === 'state-tab') {
            setTimeout(() => loadState(true), 1000); // Small delay to allow AWS to propagate changes, force fresh discovery
        }
    } else if (data.type === 'execution_aborted') {
        // New: Handle execution abort
        addGlobalExecutionLog('error', data.message || 'Execution was aborted');
        updateExecutionStatus('Execution aborted');
        stopExecutionTimer();
    }
}

function updateConnectionStatus(message, type) {
    const statusEl = document.getElementById('connectionStatus');
    const iconClass = type === 'success' ? 'fa-circle text-success' : 
                     type === 'warning' ? 'fa-exclamation-circle text-warning' : 
                     'fa-times-circle text-danger';
    
    statusEl.innerHTML = `<i class="fas ${iconClass}"></i> ${message}`;
}

// Tab Management
function openTab(evt, tabName) {
    // Hide all tab contents
    const tabContents = document.getElementsByClassName('tab-content');
    for (let i = 0; i < tabContents.length; i++) {
        tabContents[i].classList.remove('active');
    }
    
    // Remove active class from all tab buttons
    const tabButtons = document.getElementsByClassName('tab-button');
    for (let i = 0; i < tabButtons.length; i++) {
        tabButtons[i].classList.remove('active');
    }
    
    // Show selected tab and mark button as active
    document.getElementById(tabName).classList.add('active');
    evt.currentTarget.classList.add('active');
    
    currentTab = tabName;
    
    // Load content if needed
    switch (tabName) {
        case 'state-tab':
            loadState();
            break;
        case 'graph-tab':
            // Don't auto-load graph as it might be expensive
            break;
        case 'conflicts-tab':
            // Don't auto-load conflicts
            break;
        case 'plan-tab':
            // Don't auto-load plan
            break;
    }
}

// Utility Functions
function showLoader() {
    document.getElementById('loader').style.display = 'block';
}

function hideLoader() {
    document.getElementById('loader').style.display = 'none';
}

function updateStatus(message) {
    document.getElementById('statusMessage').textContent = message;
}

function showError(message) {
    updateStatus(`Error: ${message}`);
    hideLoader();
}

async function apiCall(endpoint, options = {}) {
    try {
        showLoader();
        updateStatus(`Calling ${endpoint}...`);
        
        const response = await fetch(`/api${endpoint}`, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const responseText = await response.text();
        
        // Check if response is empty
        if (!responseText) {
            throw new Error('Empty response from server');
        }
        
        // Try to parse JSON
        let data;
        try {
            data = JSON.parse(responseText);
        } catch (jsonError) {
            console.error('JSON parse error:', jsonError);
            console.error('Response text:', responseText);
            throw new Error(`Invalid JSON response: ${jsonError.message}`);
        }
        
        updateStatus('Ready');
        return data;
    } catch (error) {
        showError(error.message);
        throw error;
    } finally {
        hideLoader();
    }
}

// AI Agent Functions
async function processAgentRequest() {
    const request = document.getElementById('agentRequest').value.trim();
    if (!request) {
        showError('Please enter a request');
        return;
    }
    
    const dryRun = document.getElementById('dryRun').checked;
    
    try {
        const response = await apiCall('/agent/process', {
            method: 'POST',
            body: JSON.stringify({ request, dry_run: dryRun })
        });
        
        displayAgentResponse(response);
    } catch (error) {
        console.error('Agent request failed:', error);
    }
}

function displayAgentResponse(response) {
    console.log('Displaying agent response:', response); // Debug log
    const responseArea = document.getElementById('agentResponse');
    const resultDiv = document.getElementById('agentResult');
    
    let html = `
        <div class="alert alert-info">
            <h4><i class="fas fa-robot"></i> AI Agent Response</h4>
            <p><strong>Request:</strong> ${response.request}</p>
            <p><strong>Mode:</strong> ${response.mode === 'demo' ? 'Demo Mode' : (response.dry_run ? 'Dry Run' : 'Live Execution')}</p>
            <p><strong>Confidence:</strong> ${(response.confidence * 100).toFixed(1)}%</p>
        </div>
    `;
    
    if (response.mode === 'demo') {
        // Demo mode response
        html += `
            <div class="data-item">
                <h4>Response</h4>
                <div class="content">${response.response}</div>
            </div>
        `;
    } else if (response.mode === 'live') {
        // Live mode response - show decision and execution details
        html += `
            <div class="data-item">
                <h4>Action</h4>
                <div class="content">${response.action}</div>
            </div>
            
            <div class="data-item">
                <h4>Reasoning</h4>
                <div class="content">${response.reasoning}</div>
            </div>
        `;
        
        // Only show execution details if execution has started
        if (response.execution) {
            html += `
                <div class="data-item">
                    <h4>Execution Status</h4>
                    <div class="content">${response.execution.status}</div>
                </div>
            `;
            
            if (response.execution.actions && response.execution.actions.length > 0) {
                html += `
                    <div class="data-item">
                        <h4>Actions Performed</h4>
                        <ul>
                            ${response.execution.actions.map(action => `<li>${action}</li>`).join('')}
                        </ul>
                    </div>
                `;
            }
            
            if (response.execution.errors && response.execution.errors.length > 0) {
                html += `
                    <div class="data-item">
                        <h4>Errors</h4>
                        <ul class="error-list">
                            ${response.execution.errors.map(error => `<li>${error}</li>`).join('')}
                        </ul>
                    </div>
                `;
            }
        } else if (response.requiresConfirmation) {
            html += `
                <div class="data-item">
                    <h4>Status</h4>
                    <div class="content">Plan generated - waiting for confirmation</div>
                </div>
            `;
        }

        // Display execution plan if available
        console.log('Checking execution plan:', response.executionPlan); // Debug log
        if (response.executionPlan && response.executionPlan.length > 0) {
            console.log('Displaying execution plan with', response.executionPlan.length, 'steps'); // Debug log
            // Use setTimeout to ensure DOM is updated before showing plan
            setTimeout(() => {
                displayExecutionPlan(response.executionPlan, response.decision);
            }, 100);
        } else {
            console.log('No execution plan found in response'); // Debug log
        }
    }
    
    console.log('Setting result HTML:', html); // Debug log
    resultDiv.innerHTML = html;
    responseArea.style.display = 'block';
}

// State Management Functions
async function loadState(forceFresh = false) {
    const contentEl = document.getElementById('stateContent');
    const refreshButton = document.querySelector('button[onclick="loadState()"]');
    
    try {
        // Show loading state
        if (refreshButton) {
            refreshButton.disabled = true;
            refreshButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Refreshing...';
        }
        contentEl.innerHTML = '<div class="loading">Refreshing state...</div>';
        
        // Add parameter to force fresh discovery if needed
        // When forceFresh is true, get only discovered resources (not managed state from file)
        const url = forceFresh ? '/state?discovered_only=true' : '/state';
        console.log('Loading state from', url, '...');
        const state = await apiCall(url);
        console.log('State loaded successfully:', state);
        displayState(state);
        
        // Show success feedback
        if (refreshButton) {
            refreshButton.innerHTML = '<i class="fas fa-check"></i> Refreshed';
            setTimeout(() => {
                refreshButton.innerHTML = '<i class="fas fa-sync-alt"></i> Refresh State';
                refreshButton.disabled = false;
            }, 1000);
        }
        
    } catch (error) {
        console.error('Failed to load state:', error);
        
        // Reset button state
        if (refreshButton) {
            refreshButton.innerHTML = '<i class="fas fa-sync-alt"></i> Refresh State';
            refreshButton.disabled = false;
        }
        
        // Display error in UI
        contentEl.innerHTML = `
            <div class="error">
                <strong>Failed to load state:</strong> ${error.message}
                <br><br>
                This might indicate that the server is not running or there's a configuration issue.
                <br><br>
                <button class="btn btn-secondary" onclick="loadState()">Retry</button>
            </div>
        `;
    }
}

function displayState(state) {
    const contentEl = document.getElementById('stateContent');
    
    // Handle both direct state and managed_state structure
    let actualState = state;
    let dataSource = 'direct';
    
    // Check if we have discovered resources and managed state is empty or doesn't exist
    const hasDiscoveredResources = state.discovered_resources && 
        (Array.isArray(state.discovered_resources) ? state.discovered_resources.length > 0 : 
         Object.keys(state.discovered_resources).length > 0);
    
    const managedResourceCount = state.managed_state ? Object.keys(state.managed_state.resources || {}).length : 0;
    
    if (state.managed_state && managedResourceCount > 0) {
        // Use managed state if it has resources
        actualState = state.managed_state;
        dataSource = 'managed';
    } else if (hasDiscoveredResources) {
        // Use discovered resources if managed state is empty
        actualState = {
            resources: state.discovered_resources,
            version: '1.0',
            lastUpdated: state.timestamp || new Date().toISOString(),
            region: state.region || 'N/A'
        };
        dataSource = 'discovered';
    }
    
    // Handle string responses (JSON as string)
    if (typeof actualState === 'string') {
        try {
            actualState = JSON.parse(actualState);
        } catch (e) {
            console.error('Failed to parse state JSON:', e);
            contentEl.innerHTML = '<div class="error">Invalid state data received</div>';
            return;
        }
    }
    
    // Calculate resource count based on data source
    let resourceCount = 0;
    let resources = {};
    
    if (dataSource === 'discovered' && Array.isArray(actualState.resources)) {
        // Handle discovered resources format (array of ResourceState objects)
        // Convert array to object for consistent display
        resources = {};
        actualState.resources.forEach((resource, index) => {
            resources[resource.id || `resource-${index}`] = {
                id: resource.id,
                name: resource.name || resource.id,
                type: resource.type,
                status: resource.status,
                properties: {
                    aws_details: resource.properties
                }
            };
        });
        // Count only non-step_reference resources for display
        resourceCount = actualState.resources.filter(r => r.type !== 'step_reference').length;
    } else {
        // Handle managed state format (object with resources)
        resources = actualState.resources || {};
        // Count only non-step_reference resources for display
        resourceCount = Object.values(resources).filter(r => r.type !== 'step_reference').length;
    }
    
    let html = `
        <div class="stats-grid">
            <div class="stat-card">
                <span class="number">${resourceCount}</span>
                <span class="label">${dataSource === 'discovered' ? 'Discovered Resources' : 'Managed Resources'}</span>
            </div>
            <div class="stat-card">
                <span class="number">${actualState.version || '1.0.0'}</span>
                <span class="label">State Version</span>
            </div>
            <div class="stat-card">
                <span class="number">${new Date(actualState.lastUpdated || actualState.last_updated || Date.now()).toLocaleDateString()}</span>
                <span class="label">Last Updated</span>
            </div>
            <div class="stat-card">
                <span class="number">${actualState.region || 'N/A'}</span>
                <span class="label">Region</span>
            </div>
        </div>
    `;
    
    if (dataSource === 'discovered') {
        html += '<div class="alert alert-info"><i class="fas fa-cloud"></i> Showing live AWS resources (no managed state found)</div>';
    }
    
    if (actualState.error) {
        html += `<div class="error">Error: ${actualState.error}</div>`;
    }
    
    if (resourceCount > 0) {
        html += '<div class="data-grid">';
        for (const [id, resource] of Object.entries(resources)) {
            // Skip step_reference resources as they are internal for dependency resolution
            if (resource.type === 'step_reference') {
                continue;
            }
            
            // Extract AWS resource ID from different formats
            let awsResourceId = 'N/A';
            if (dataSource === 'discovered' && resource.properties && resource.properties.aws_details) {
                // For discovered resources
                const details = resource.properties.aws_details;
                awsResourceId = details.instanceId || details.vpcId || details.subnetId || 
                               details.groupId || details.loadBalancerArn || resource.id || 'N/A';
            } else if (resource.properties && resource.properties.mcp_response) {
                // For managed resources
                const mcpResponse = resource.properties.mcp_response;
                awsResourceId = mcpResponse.groupId || mcpResponse.instanceId || 
                               mcpResponse.vpcId || mcpResponse.subnetId || 'N/A';
            }
            
            html += `
                <div class="data-item">
                    <h4>${id}</h4>
                    <div class="meta">
                        <span class="badge type-${resource.type}">${resource.type}</span>
                        <span class="badge status-${resource.status}">${resource.status}</span>
                        ${awsResourceId !== 'N/A' ? `<span class="badge aws-id">AWS: ${awsResourceId}</span>` : ''}
                    </div>
                    <div class="details">
                        <div class="detail-row">
                            <strong>Name:</strong> ${resource.name || 'Unnamed'}
                        </div>
                        <div class="detail-row">
                            <strong>Created:</strong> ${new Date(resource.createdAt || Date.now()).toLocaleString()}
                        </div>
                        ${resource.dependencies && resource.dependencies.length > 0 ? 
                            `<div class="detail-row"><strong>Dependencies:</strong> ${resource.dependencies.join(', ')}</div>` : ''
                        }
                    </div>
                    <details class="properties-details">
                        <summary>Properties</summary>
                        <pre class="properties-content">${JSON.stringify(resource.properties, null, 2)}</pre>
                    </details>
                </div>
            `;
        }
        html += '</div>';
    } else {
        html += '<div class="placeholder">No managed resources found. Create some infrastructure to see it here.</div>';
    }
    
    contentEl.innerHTML = html;
}

async function discoverInfrastructure() {
    try {
        const result = await apiCall('/discover', { method: 'POST' });
        displayDiscoveredResources(result);
    } catch (error) {
        console.error('Discovery failed:', error);
    }
}

function displayDiscoveredResources(result) {
    const contentEl = document.getElementById('stateContent');
    
    let html = `
        <div class="alert alert-success">
            <h4><i class="fas fa-search"></i> Infrastructure Discovery Complete</h4>
            <p>Found ${result.count} resources at ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.resources && result.resources.length > 0) {
        html += '<div class="data-grid">';
        result.resources.forEach(resource => {
            html += `
                <div class="data-item">
                    <h4>${resource.id}</h4>
                    <div class="meta">Type: ${resource.type} | Status: ${resource.status}</div>
                    <div class="content">${JSON.stringify(resource.properties, null, 2)}</div>
                </div>
            `;
        });
        html += '</div>';
    } else {
        html += '<div class="placeholder">No resources discovered</div>';
    }
    
    contentEl.innerHTML = html;
}

async function exportState() {
    try {
        const includeDiscovered = confirm('Include discovered (unmanaged) resources in export?');
        const url = `/api/export?include_discovered=${includeDiscovered}`;
        
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`Export failed: ${response.statusText}`);
        }
        
        const blob = await response.blob();
        const downloadUrl = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = downloadUrl;
        a.download = `infrastructure-state-${new Date().toISOString().split('T')[0]}.json`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(downloadUrl);
        
        updateStatus('State exported successfully');
    } catch (error) {
        showError(`Export failed: ${error.message}`);
    }
}

// Graph Functions
async function loadGraph() {
    try {
        const format = document.getElementById('graphFormat').value;
        const result = await apiCall(`/graph?format=${format}`);
        displayGraph(result);
    } catch (error) {
        console.error('Graph generation failed:', error);
    }
}

function displayGraph(result) {
    const contentEl = document.getElementById('graphContent');
    
    let html = `
        <div class="alert alert-info">
            <h4><i class="fas fa-project-diagram"></i> Dependency Graph</h4>
            <p>Format: ${result.format} | Generated: ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.format === 'mermaid') {
        html += `
            <div class="mermaid">
                ${result.visualization}
            </div>
        `;
    } else {
        html += `
            <div class="graph-text">${result.visualization}</div>
        `;
    }
    
    if (result.bottlenecks && result.bottlenecks.length > 0) {
        html += `
            <div class="alert alert-warning">
                <h4><i class="fas fa-exclamation-triangle"></i> Bottlenecks Detected</h4>
                <ul>
                    ${result.bottlenecks.map(b => `<li>${b}</li>`).join('')}
                </ul>
            </div>
        `;
    }
    
    contentEl.innerHTML = html;
    
    // Re-render mermaid diagrams
    if (result.format === 'mermaid') {
        mermaid.init(undefined, contentEl.querySelector('.mermaid'));
    }
}

// Conflicts Functions
async function loadConflicts() {
    try {
        const autoResolve = document.getElementById('autoResolve').checked;
        const result = await apiCall(`/conflicts?auto_resolve=${autoResolve}`);
        displayConflicts(result);
    } catch (error) {
        console.error('Conflict detection failed:', error);
    }
}

function displayConflicts(result) {
    const contentEl = document.getElementById('conflictsContent');
    
    let html = `
        <div class="alert ${result.count > 0 ? 'alert-warning' : 'alert-success'}">
            <h4><i class="fas fa-exclamation-triangle"></i> Conflict Analysis</h4>
            <p>Found ${result.count} conflicts | Analyzed: ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.conflicts && result.conflicts.length > 0) {
        html += '<div class="data-grid">';
        result.conflicts.forEach(conflict => {
            html += `
                <div class="data-item conflict-item">
                    <h4>Conflict: ${conflict.resource_id}</h4>
                    <div class="meta">Type: ${conflict.type} | Severity: ${conflict.severity}</div>
                    <div class="content">${conflict.description}</div>
                </div>
            `;
        });
        html += '</div>';
    }
    
    if (result.resolutions && result.resolutions.length > 0) {
        html += '<h3 style="margin: 20px; color: #28a745;">Auto-Resolutions</h3>';
        html += '<div class="data-grid">';
        result.resolutions.forEach(resolution => {
            html += `
                <div class="data-item resolution-item">
                    <h4>Resolution: ${resolution.conflict.resource_id}</h4>
                    <div class="meta">Action: ${resolution.resolution.action}</div>
                    <div class="content">${resolution.resolution.description}</div>
                </div>
            `;
        });
        html += '</div>';
    }
    
    if (result.count === 0) {
        html += '<div class="placeholder">No conflicts detected</div>';
    }
    
    contentEl.innerHTML = html;
}

// Planning Functions
async function generatePlan() {
    try {
        const includeLevels = document.getElementById('includeLevels').checked;
        const result = await apiCall('/plan', {
            method: 'POST',
            body: JSON.stringify({ include_levels: includeLevels })
        });
        displayPlan(result);
    } catch (error) {
        console.error('Plan generation failed:', error);
    }
}

function displayPlan(result) {
    const contentEl = document.getElementById('planContent');
    
    let html = `
        <div class="alert alert-info">
            <h4><i class="fas fa-calendar-alt"></i> Deployment Plan</h4>
            <p>Resources: ${result.resource_count} | Generated: ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.deployment_order && result.deployment_order.length > 0) {
        html += `
            <div class="data-item">
                <h4>Deployment Order</h4>
                <div class="content">
                    ${result.deployment_order.map((resource, index) => 
                        `${index + 1}. ${resource}`
                    ).join('\n')}
                </div>
            </div>
        `;
    }
    
    if (result.deployment_levels) {
        html += `
            <div class="data-item">
                <h4>Deployment Levels (Parallel Groups)</h4>
                <div class="content">
                    ${Object.entries(result.deployment_levels).map(([level, resources]) => 
                        `Level ${level}: ${resources.join(', ')}`
                    ).join('\n')}
                </div>
            </div>
        `;
    }
    
    if (!result.deployment_order || result.deployment_order.length === 0) {
        html += '<div class="placeholder">No deployment plan available</div>';
    }
    
    contentEl.innerHTML = html;
}

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    // Ctrl+Enter to process agent request
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        if (currentTab === 'agent-tab') {
            processAgentRequest();
        }
    }
    
    // F5 to refresh current tab content
    if (e.key === 'F5') {
        e.preventDefault();
        switch (currentTab) {
            case 'state-tab':
                loadState();
                break;
            case 'graph-tab':
                loadGraph();
                break;
            case 'conflicts-tab':
                loadConflicts();
                break;
            case 'plan-tab':
                generatePlan();
                break;
        }
    }
});

// Execution Plan Functions
let currentPlan = null;
let currentDecision = null;
let executionStartTime = null;
let executionTimer = null;

function displayExecutionPlan(executionPlan, decision) {
    currentPlan = executionPlan;
    currentDecision = decision;
    
    const planDiv = document.getElementById('executionPlan');
    const stepsDiv = document.getElementById('planSteps');
    const confirmBtn = document.getElementById('confirmPlanBtn');
    
    // Create integrated execution plan with progress tracking
    let html = `
        <div class="execution-header">
            <div class="execution-status-bar">
                <div class="execution-info">
                    <span id="executionStatus">Ready to execute</span>
                    <span id="executionTimer">00:00</span>
                </div>
                <div class="execution-progress-bar">
                    <div id="progressFill" class="progress-fill" style="width: 0%"></div>
                </div>
            </div>
        </div>
    `;
    
    // Add each step with integrated logs and status
    executionPlan.forEach((step, index) => {
        html += `
            <div class="integrated-step" data-step-id="${step.id}" data-step-index="${index}">
                <div class="step-header">
                    <div class="step-number">${index + 1}</div>
                    <div class="step-info">
                        <div class="step-name">${step.name}</div>
                        <div class="step-description">${step.description}</div>
                    </div>
                    <div class="step-controls">
                        <div class="step-status pending"></div>
                        <button class="expand-logs" onclick="toggleStepLogs('${step.id}')" style="display: none;">
                            <i class="fas fa-chevron-down"></i>
                        </button>
                    </div>
                </div>
                <div class="step-logs" id="logs-${step.id}" style="display: none;">
                    <div class="logs-content"></div>
                </div>
            </div>
        `;
    });
    
    stepsDiv.innerHTML = html;
    confirmBtn.style.display = 'inline-block';
    planDiv.style.display = 'block';
}

async function confirmAndExecutePlan() {
    if (!currentDecision) {
        showError('No decision available for execution');
        return;
    }
    
    try {
        // Don't hide the plan - keep it visible for integrated progress tracking
        // Hide only the confirm button and show execution status
        document.getElementById('confirmPlanBtn').style.display = 'none';
        
        // Update execution status
        updateExecutionStatus('Execution starting...');
        
        // Initialize execution tracking
        executionStartTime = Date.now();
        startExecutionTimer();
        
        // Call execute API
        const response = await apiCall('/agent/execute', {
            method: 'POST',
            body: JSON.stringify({ decisionId: currentDecision.id })
        });
        
        if (response.success) {
            addGlobalExecutionLog('info', `Plan execution started: ${response.executionId}`);
            updateExecutionStatus('Execution in progress...');
        } else {
            addGlobalExecutionLog('error', 'Failed to start execution');
            updateExecutionStatus('Execution failed to start');
        }
        
    } catch (error) {
        console.error('Failed to execute plan:', error);
        addGlobalExecutionLog('error', `Execution failed: ${error.message}`);
        updateExecutionStatus('Execution error');
    }
}

function cancelPlan() {
    document.getElementById('executionPlan').style.display = 'none';
    currentPlan = null;
    currentDecision = null;
}

function updateExecutionStatus(status) {
    document.getElementById('executionStatus').textContent = status;
}

function updateExecutionProgress(percentage) {
    const progressFill = document.getElementById('progressFill');
    if (progressFill) {
        progressFill.style.width = `${percentage}%`;
    }
}

function startExecutionTimer() {
    const timerElement = document.getElementById('executionTimer');
    
    executionTimer = setInterval(() => {
        if (executionStartTime) {
            const elapsed = Date.now() - executionStartTime;
            const minutes = Math.floor(elapsed / 60000);
            const seconds = Math.floor((elapsed % 60000) / 1000);
            timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        }
    }, 1000);
}

function stopExecutionTimer() {
    if (executionTimer) {
        clearInterval(executionTimer);
        executionTimer = null;
    }
}

function updateStepStatus(stepId, status) {
    const stepElement = document.querySelector(`[data-step-id="${stepId}"]`);
    if (stepElement) {
        const statusElement = stepElement.querySelector('.step-status');
        const expandButton = stepElement.querySelector('.expand-logs');
        
        if (statusElement) {
            // Add a subtle fade transition when changing status
            statusElement.style.opacity = '0.7';
            setTimeout(() => {
                // Remove old classes and add new status
                statusElement.className = `step-status ${status}`;
                // Don't set text content - let CSS handle icons
                statusElement.textContent = '';
                statusElement.style.opacity = '1';
            }, 150);
        }
        
        // Show expand button when step is running or has logs
        if (expandButton && (status === 'running' || status === 'completed' || status === 'failed' || status.includes('recovery'))) {
            expandButton.style.display = 'block';
        }
        
        // Auto-expand on failure or recovery
        if (status === 'failed' || status.includes('recovery')) {
            toggleStepLogs(stepId, true);
        }
    }
}

// Function to toggle step logs visibility
function toggleStepLogs(stepId, forceOpen = false) {
    const logsDiv = document.getElementById(`logs-${stepId}`);
    const expandButton = document.querySelector(`[data-step-id="${stepId}"] .expand-logs i`);
    
    if (logsDiv) {
        const isHidden = logsDiv.style.display === 'none' || logsDiv.style.display === '';
        
        if (forceOpen || isHidden) {
            logsDiv.style.display = 'block';
            if (expandButton) expandButton.className = 'fas fa-chevron-up';
        } else {
            logsDiv.style.display = 'none';
            if (expandButton) expandButton.className = 'fas fa-chevron-down';
        }
    }
}

// Add log to specific step
function addStepLog(stepId, level, message) {
    const stepLogsDiv = document.querySelector(`#logs-${stepId} .logs-content`);
    if (stepLogsDiv) {
        const timestamp = new Date().toLocaleTimeString();
        const logEntry = document.createElement('div');
        logEntry.className = `step-log-entry level-${level}`;
        logEntry.innerHTML = `
            <span class="timestamp">${timestamp}</span>
            <span class="level">[${level.toUpperCase()}]</span>
            <span class="message">${message}</span>
        `;
        stepLogsDiv.appendChild(logEntry);
        
        // Auto-scroll to bottom
        stepLogsDiv.scrollTop = stepLogsDiv.scrollHeight;
        
        // Show expand button if not visible
        const stepElement = document.querySelector(`[data-step-id="${stepId}"]`);
        const expandButton = stepElement?.querySelector('.expand-logs');
        if (expandButton) {
            expandButton.style.display = 'block';
        }
    }
}

// Add global execution log (for general execution events)
function addGlobalExecutionLog(level, message) {
    console.log(`[${level.toUpperCase()}] ${message}`);
}

// Recovery Functions - Inline UX
let currentRecoveryContext = null;

function showRecoveryDialog(failureContext, recoveryOptions, stepId) {
    console.log('showRecoveryDialog called with:', {
        failureContext,
        recoveryOptions, 
        stepId
    });
    
    try {
        // Store recovery context
        currentRecoveryContext = {
            failureContext,
            recoveryOptions,
            stepId
        };
        
        // Find the failed step element in the integrated plan
        const stepElement = document.querySelector(`[data-step-id="${stepId}"]`);
        if (!stepElement) {
            console.error('Could not find step element for stepId:', stepId);
            // Fallback: add recovery info to step logs
            addStepLog(stepId, 'recovery', 'Recovery options available - please refresh page');
            return;
        }
        
        // Ensure the step logs are visible
        toggleStepLogs(stepId, true);
        
        // Create inline recovery options HTML
        const recoveryHTML = createInlineRecoveryHTML(failureContext, recoveryOptions, stepId);
        
        // Insert recovery options after the step
        const recoveryContainer = document.createElement('div');
        recoveryContainer.className = 'inline-recovery-container';
        
        // Safely set innerHTML with error handling
        try {
            recoveryContainer.innerHTML = recoveryHTML;
        } catch (htmlError) {
            console.error('Failed to set recovery HTML:', htmlError);
            recoveryContainer.innerHTML = '<div class="recovery-error">Failed to display recovery options. Please refresh the page.</div>';
        }
        
        // Insert after the step element (integrated step design)
        try {
            stepElement.parentNode.insertBefore(recoveryContainer, stepElement.nextSibling);
        } catch (insertError) {
            console.error('Failed to insert recovery container:', insertError);
            // Fallback: add to step logs
            addStepLog(stepId, 'recovery', 'Recovery needed - please refresh page to see options');
            return;
        }
        
        // Set up event handlers for the recovery options
        try {
            setupInlineRecoveryHandlers(recoveryContainer);
        } catch (handlerError) {
            console.error('Failed to setup recovery handlers:', handlerError);
        }
        
        console.log('Inline recovery options displayed for step:', stepId);
        
    } catch (error) {
        console.error('Error in showRecoveryDialog:', error);
        // Ultimate fallback: add to step logs and show alert
        addStepLog(stepId, 'error', `Recovery error: ${error.message}`);
        alert(`Step ${stepId} failed. Please refresh the page and try again. Error: ${failureContext?.errorMessage || 'Unknown error'}`);
    }
}

function createInlineRecoveryHTML(failureContext, recoveryOptions, stepId) {
    const aiAnalysisSection = failureContext.aiAnalysis ? `
        <div class="recovery-analysis">
            <div class="analysis-item">
                <strong>Root Cause:</strong> ${escapeHtml(failureContext.aiAnalysis.rootCause || 'Not available')}
            </div>
            <div class="analysis-item">
                <strong>Recommendation:</strong> ${escapeHtml(failureContext.aiAnalysis.recommendation || 'Not available')}
            </div>
        </div>
    ` : '';
    
    const optionsHTML = recoveryOptions && recoveryOptions.length > 0 ? 
        recoveryOptions.map((option, index) => {
            const successProb = Math.round((option.successProbability || 0) * 100);
            const riskLevel = (option.riskLevel || 'medium').toLowerCase();
            
            // Check if this is a multi-step recovery option
            const isMultiStep = option.action === 'multi_step_recovery' && option.multiStepPlan && option.multiStepPlan.length > 0;
            const multiStepDetails = isMultiStep ? `
                <div class="multi-step-details">
                    <div class="multi-step-header">
                        <i class="fas fa-list-ol"></i>
                        <span>Multi-Step Recovery Plan (${option.totalSteps || option.multiStepPlan.length} steps)</span>
                        <button class="toggle-steps-btn" type="button">
                            <i class="fas fa-chevron-down"></i>
                        </button>
                    </div>
                    <div class="multi-step-list" style="display: none;">
                        ${option.multiStepPlan.map((step, stepIndex) => `
                            <div class="recovery-step-item">
                                <div class="step-number">${step.stepOrder || stepIndex + 1}</div>
                                <div class="step-content">
                                    <div class="step-tool">${escapeHtml(step.toolName)}</div>
                                    <div class="step-purpose">${escapeHtml(step.purpose)}</div>
                                    ${step.parameters && Object.keys(step.parameters).length > 0 ? `
                                        <div class="step-params">
                                            <span class="params-label">Parameters:</span>
                                            <span class="params-value">${escapeHtml(JSON.stringify(step.parameters, null, 2))}</span>
                                        </div>
                                    ` : ''}
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
            ` : '';
            
            return `
                <div class="inline-recovery-option ${isMultiStep ? 'multi-step-option' : ''}" data-option-index="${index}">
                    <div class="option-header">
                        <div class="option-action">
                            ${escapeHtml(option.action || 'Unknown action')}
                            ${isMultiStep ? `<span class="multi-step-badge">${option.totalSteps || option.multiStepPlan.length} steps</span>` : ''}
                        </div>
                        <div class="option-metrics">
                            <span class="success-prob">${successProb}% Success</span>
                            <span class="risk-level ${riskLevel}">${escapeHtml(option.riskLevel || 'Medium')} Risk</span>
                        </div>
                    </div>
                    <div class="option-reasoning">${escapeHtml(option.reasoning || 'No reasoning provided')}</div>
                    ${!isMultiStep && (option.newTool || option.modifiedParameters) ? `
                        <div class="option-details">
                            ${option.newTool ? `New Tool: ${escapeHtml(option.newTool)}` : ''}
                            ${option.modifiedParameters ? ` | Parameters: ${escapeHtml(JSON.stringify(option.modifiedParameters))}` : ''}
                        </div>
                    ` : ''}
                    ${multiStepDetails}
                </div>
            `;
        }).join('') : '<div class="no-options">No recovery options available</div>';
    
    return `
        <div class="recovery-step">
            <div class="recovery-header">
                <i class="fas fa-exclamation-triangle"></i>
                <span class="recovery-title">Step Failed - Recovery Options Available</span>
            </div>
            
            <div class="failure-details">
                <div class="failure-info">
                    <strong>Error:</strong> ${escapeHtml(failureContext.errorMessage || 'Unknown error')}
                </div>
                <div class="step-info">
                    <strong>Step:</strong> ${escapeHtml(failureContext.stepName || 'Unknown')} | 
                    <strong>Tool:</strong> ${escapeHtml(failureContext.toolName || 'Unknown')}
                </div>
            </div>
            
            ${aiAnalysisSection}
            
            <div class="recovery-options-section">
                <div class="options-header">Choose Recovery Action:</div>
                <div class="recovery-options-list">
                    ${optionsHTML}
                    
                    <div class="inline-recovery-option" data-option-index="skip">
                        <div class="option-header">
                            <div class="option-action">Skip This Step</div>
                            <div class="option-metrics">
                                <span class="risk-level medium">Medium Risk</span>
                            </div>
                        </div>
                        <div class="option-reasoning">Skip this step and continue with the next step in the plan.</div>
                        <div class="option-details">Note: May affect steps that depend on this step's output.</div>
                    </div>
                </div>
            </div>
            
            <div class="recovery-actions">
                <button type="button" class="btn-recovery btn-abort" onclick="abortExecution()">
                    <i class="fas fa-times"></i> Abort Execution
                </button>
                <button type="button" class="btn-recovery btn-proceed" disabled data-step-id="${stepId}">
                    <i class="fas fa-play"></i> Proceed with Selected Option
                </button>
            </div>
        </div>
    `;
}

function setupInlineRecoveryHandlers(container) {
    const options = container.querySelectorAll('.inline-recovery-option');
    const proceedButton = container.querySelector('.btn-proceed');
    
    console.log('Setting up inline recovery handlers:', {
        optionsFound: options.length,
        proceedButtonFound: !!proceedButton
    });
    
    options.forEach(option => {
        option.addEventListener('click', function(e) {
            // Don't trigger selection if clicking on toggle button
            if (e.target.closest('.toggle-steps-btn')) {
                return;
            }
            
            // Remove selected class from all options in this container
            options.forEach(opt => opt.classList.remove('selected'));
            
            // Add selected class to clicked option
            this.classList.add('selected');
            
            // Enable proceed button
            if (proceedButton) {
                proceedButton.disabled = false;
            }
            
            console.log('Recovery option selected:', this.getAttribute('data-option-index'));
        });
    });
    
    // Set up toggle handlers for multi-step details
    const toggleButtons = container.querySelectorAll('.toggle-steps-btn');
    toggleButtons.forEach(button => {
        button.addEventListener('click', function(e) {
            e.stopPropagation(); // Prevent option selection
            
            const stepsList = this.closest('.multi-step-details').querySelector('.multi-step-list');
            const icon = this.querySelector('i');
            
            if (stepsList.style.display === 'none' || stepsList.style.display === '') {
                stepsList.style.display = 'block';
                icon.className = 'fas fa-chevron-up';
                this.closest('.multi-step-details').classList.add('expanded');
            } else {
                stepsList.style.display = 'none';
                icon.className = 'fas fa-chevron-down';
                this.closest('.multi-step-details').classList.remove('expanded');
            }
        });
    });
    
    // Set up proceed button handler
    if (proceedButton) {
        proceedButton.addEventListener('click', function() {
            const selectedOption = container.querySelector('.inline-recovery-option.selected');
            if (!selectedOption) {
                alert('Please select a recovery option');
                return;
            }
            
            const optionIndex = selectedOption.getAttribute('data-option-index');
            const stepId = this.getAttribute('data-step-id');
            
            proceedWithRecovery(optionIndex, stepId, container);
        });
    }
}

function updateMultiStepProgress(recoveryContainer, progressMessage, stepId) {
    // Check if this is a recovery-in-progress container
    const progressContainer = recoveryContainer.querySelector('.recovery-in-progress');
    if (!progressContainer) {
        return; // Not in progress state
    }
    
    // Update or create progress details section
    let progressDetails = progressContainer.querySelector('.multi-step-progress');
    if (!progressDetails) {
        progressDetails = document.createElement('div');
        progressDetails.className = 'multi-step-progress';
        progressContainer.appendChild(progressDetails);
    }
    
    // Extract step info from progress message (format: "Recovery step X/Y: purpose")
    const stepMatch = progressMessage.match(/Recovery step (\d+)\/(\d+): (.+)/);
    if (stepMatch) {
        const currentStep = stepMatch[1];
        const totalSteps = stepMatch[2];
        const stepPurpose = stepMatch[3];
        
        // Update progress display
        progressDetails.innerHTML = `
            <div class="progress-step-info">
                <div class="progress-step-number">Step ${currentStep} of ${totalSteps}</div>
                <div class="progress-step-purpose">${escapeHtml(stepPurpose)}</div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${(currentStep / totalSteps) * 100}%"></div>
                </div>
            </div>
        `;
        
        // Update the recovery title to reflect current step
        const recoveryTitle = progressContainer.querySelector('.recovery-title');
        if (recoveryTitle) {
            recoveryTitle.textContent = `Executing Recovery Step ${currentStep}/${totalSteps}`;
        }
    } else {
        // Fallback for generic progress messages
        progressDetails.innerHTML = `
            <div class="progress-step-info">
                <div class="progress-step-purpose">${escapeHtml(progressMessage)}</div>
            </div>
        `;
    }
}

function proceedWithRecovery(optionIndex, stepId, container) {
    console.log('Proceeding with recovery:', { optionIndex, stepId });
    
    // Show recovery progress inline
    container.innerHTML = `
        <div class="recovery-step recovery-in-progress">
            <div class="recovery-header">
                <i class="fas fa-sync-alt fa-spin"></i>
                <span class="recovery-title">Applying Recovery Strategy...</span>
            </div>
            <div class="recovery-progress-info">
                Selected: ${optionIndex === 'skip' ? 'Skip Step' : `Option ${parseInt(optionIndex) + 1}`}
            </div>
        </div>
    `;
    
    // Send recovery decision to server via WebSocket
    if (websocket && websocket.readyState === WebSocket.OPEN) {
        const message = {
            type: 'recovery_decision',
            stepId: stepId,
            selectedOptionIndex: optionIndex,
            timestamp: new Date().toISOString()
        };
        
        websocket.send(JSON.stringify(message));
        addGlobalExecutionLog('info', `Recovery option selected: ${optionIndex === 'skip' ? 'Skip Step' : 'Option ' + (parseInt(optionIndex) + 1)}`);
    } else {
        // Fallback to HTTP API if WebSocket is not available
        sendRecoveryDecisionHTTP(stepId, optionIndex);
    }
    
    // Reset recovery context
    currentRecoveryContext = null;
}

// Helper function to escape HTML to prevent XSS
function escapeHtml(unsafe) {
    if (typeof unsafe !== 'string') return String(unsafe || '');
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function abortExecution() {
    const stepId = currentRecoveryContext ? currentRecoveryContext.stepId : null;
    
    // Remove any inline recovery containers
    const recoveryContainers = document.querySelectorAll('.inline-recovery-container');
    recoveryContainers.forEach(container => container.remove());
    
    // Send abort decision to server via WebSocket
    if (websocket && websocket.readyState === WebSocket.OPEN) {
        const message = {
            type: 'recovery_abort',
            stepId: stepId,
            timestamp: new Date().toISOString()
        };
        
        websocket.send(JSON.stringify(message));
        addGlobalExecutionLog('error', 'Execution aborted by user during recovery');
    } else {
        // Fallback to HTTP API if WebSocket is not available
        sendRecoveryAbortHTTP(stepId);
    }
    
    // Reset recovery context
    currentRecoveryContext = null;
    
    // Update execution status
    updateExecutionStatus('Execution aborted');
    stopExecutionTimer();
}

async function sendRecoveryDecisionHTTP(stepId, optionIndex) {
    try {
        await apiCall('/agent/recovery-decision', {
            method: 'POST',
            body: JSON.stringify({
                stepId: stepId,
                selectedOptionIndex: optionIndex
            })
        });
    } catch (error) {
        console.error('Failed to send recovery decision:', error);
        addGlobalExecutionLog('error', 'Failed to communicate recovery decision to server');
    }
}

async function sendRecoveryAbortHTTP(stepId) {
    try {
        await apiCall('/agent/recovery-abort', {
            method: 'POST',
            body: JSON.stringify({
                stepId: stepId
            })
        });
    } catch (error) {
        console.error('Failed to send recovery abort:', error);
        addGlobalExecutionLog('error', 'Failed to communicate recovery abort to server');
    }
}
