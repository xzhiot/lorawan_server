// 简单缓存系统
const simpleCache = {
  data: {},
  set(key, value, ttl = 300000) { // 5分钟默认
    this.data[key] = {
      value,
      expires: Date.now() + ttl
    };
  },
  get(key) {
    const item = this.data[key];
    if (!item) return null;
    if (Date.now() > item.expires) {
      delete this.data[key];
      return null;
    }
    return item.value;
  },
  clear() {
    this.data = {};
  }
};

// 辅助函数：格式化设备状态
function formatDeviceStatus(device) {
  // 检查设备是否已激活
  if (
    !device.devAddr ||
    (Array.isArray(device.devAddr) && device.devAddr.length === 0)
  ) {
    return {
      status: "not-activated",
      text: "未激活",
      class: "status-inactive",
    };
  }

  // 检查是否有发送过数据
  if (!device.lastSeenAt && device.fCntUp === 0) {
    return {
      status: "activated",
      text: "已激活 (无数据)",
      class: "status-warning",
    };
  }

  // 如果有 lastSeenAt，计算时间差
  const lastTime = device.lastSeenAt || device.updatedAt;
  if (lastTime) {
    const lastSeen = new Date(lastTime);
    const now = new Date();
    const diffMinutes = Math.floor((now - lastSeen) / 1000 / 60);

    if (diffMinutes < 5) {
      return {
        status: "online",
        text: "在线",
        class: "status-active",
      };
    } else if (diffMinutes < 60) {
      return {
        status: "recent",
        text: `${diffMinutes}分钟前`,
        class: "status-warning",
      };
    } else if (diffMinutes < 1440) {
      // 24 hours
      const hours = Math.floor(diffMinutes / 60);
      return {
        status: "inactive",
        text: `${hours}小时前`,
        class: "status-inactive",
      };
    } else {
      const days = Math.floor(diffMinutes / 1440);
      return {
        status: "offline",
        text: `${days}天前`,
        class: "status-error",
      };
    }
  }

  return {
    status: "unknown",
    text: "未知",
    class: "status-inactive",
  };
}

// 辅助函数：格式化设备地址
function formatDevAddr(devAddr) {
  if (!devAddr) return null;

  // 如果是数组格式 [207, 156, 241, 123]
  if (Array.isArray(devAddr)) {
    return devAddr
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("")
      .toUpperCase();
  }

  // 如果已经是字符串
  return devAddr;
}

// 添加快速发送测试下行数据
async function sendTestDownlink(devEUI) {
  const testPayload = prompt("输入测试数据 (16进制):", "01020304");
  if (!testPayload) return;

  try {
    await apiRequest("POST", `/devices/${devEUI}/downlink`, {
      fPort: 1,
      data: testPayload,
      confirmed: false,
    });

    showNotification("测试下行数据已发送", "success");
  } catch (error) {
    console.error("Failed to send test downlink:", error);
    showNotification("发送失败", "error");
  }
}

// API Configuration
const API_BASE =
  window.location.hostname === "localhost"
    ? "http://localhost:8097/api/v1"
    : "/api/v1";

// Check authentication
if (
  !localStorage.getItem("access_token") &&
  !window.location.pathname.includes("login")
) {
  window.location.href = "/login.html";
}

// Global state
let currentPage = "dashboard";
let applications = [];
let currentApplication = null;

// API Helper
async function apiRequest(method, endpoint, body = null) {
  const options = {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${localStorage.getItem("access_token")}`,
    },
  };

  if (body) {
    options.body = JSON.stringify(body);
  }

  try {
    const response = await fetch(API_BASE + endpoint, options);

    if (response.status === 401) {
      // Token expired
      localStorage.clear();
      window.location.href = "/login.html";
      return;
    }

    const data = await response.json();

    if (!response.ok) {
      throw new Error(data.error || "API request failed");
    }

    return data;
  } catch (error) {
    console.error("API Error:", error);
    showNotification(error.message, "error");
    throw error;
  }
}

// 自动刷新间隔配置
const AUTO_REFRESH_INTERVALS = {
  dashboard: 30000,  // 30秒
  events: 10000,     // 10秒 - 事件页面更频繁
};

let autoRefreshTimer = null;

// 设置自动刷新
function setupAutoRefresh() {
  // 清除之前的定时器
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
    autoRefreshTimer = null;
  }
  
  // 获取当前页面的刷新间隔
  const interval = AUTO_REFRESH_INTERVALS[currentPage];
  
  if (interval) {
    console.log(`设置 ${currentPage} 页面自动刷新，间隔: ${interval/1000}秒`);
    
    autoRefreshTimer = setInterval(() => {
      if (currentPage === "dashboard") {
        loadDashboard();
      } else if (currentPage === "events") {
        loadEvents();  // 直接调用简单刷新
      }
    }, interval);
  }
}

// Page Navigation
function showPage(page) {
  // Hide all pages
  document.querySelectorAll(".page").forEach((p) => p.classList.add("hidden"));

  // Show selected page
  document.getElementById(`${page}-page`).classList.remove("hidden");

  // Update nav
  document
    .querySelectorAll(".nav-menu li")
    .forEach((li) => li.classList.remove("active"));
  event.target.closest("li").classList.add("active");

  currentPage = page;
  setupAutoRefresh();
  
  // Load page data
  switch (page) {
    case "dashboard":
      loadDashboard();
      break;
    case "applications":
      loadApplications();
      break;
    case "devices":
      loadDevices();
      break;
    case "gateways":
      loadGateways();
      break;
    case "events":
      loadEvents();
      break;
    case "settings":
      loadSettings();
      break;
    case "users":
      loadUsers();
      break;
  }
}

// Dashboard
async function loadDashboard() {
  try {
    // 加载真实统计数据
    await loadDashboardStats();

    // Load recent activity
    const events = await apiRequest("GET", "/events?limit=10");
    const tbody = document.getElementById("activity-table");

    if (events.events && events.events.length > 0) {
      tbody.innerHTML = events.events
        .map(
          (event) => `
                <tr>
                    <td>${new Date(event.createdAt).toLocaleString()}</td>
                    <td>${event.devEUI || event.gatewayId || "-"}</td>
                    <td>${event.type}</td>
                    <td>${event.description}</td>
                    <td><span class="status-${event.level.toLowerCase()}">${
            event.level
          }</span></td>
                </tr>
            `
        )
        .join("");
    } else {
      tbody.innerHTML =
        '<tr><td colspan="5" style="text-align: center;">No recent activity</td></tr>';
    }
  } catch (error) {
    console.error("Failed to load dashboard:", error);
  }
}

// 新增函数 - 加载仪表板统计数据
async function loadDashboardStats() {
  try {
    // 先获取所有应用
    const appsData = await apiRequest("GET", "/applications");
    const apps = appsData.applications || [];

    // 获取网关数据
    const gatewaysData = await apiRequest("GET", "/gateways");
    const gateways = gatewaysData.gateways || [];

    // 获取所有应用的设备
    let allDevices = [];
    for (const app of apps) {
      try {
        const devicesData = await apiRequest(
          "GET",
          `/devices?application_id=${app.id}`
        );
        if (devicesData.devices) {
          allDevices = allDevices.concat(devicesData.devices);
        }
      } catch (error) {
        console.error(`Failed to load devices for app ${app.id}:`, error);
      }
    }

    // 获取24小时内的所有上行事件来统计活跃设备
    const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
    const activeEventsData = await apiRequest(
      "GET",
      "/events?type=UPLINK&created_after=" + oneDayAgo.toISOString()
    );

    // 从事件中提取唯一的活跃设备EUI
    const activeDeviceEUIs = new Set();
    if (activeEventsData.events) {
      activeEventsData.events.forEach((event) => {
        if (event.devEUI) {
          activeDeviceEUIs.add(event.devEUI);
        }
      });
    }

    // 获取今日消息数
    const todayStart = new Date();
    todayStart.setHours(0, 0, 0, 0);
    const todayEventsData = await apiRequest(
      "GET",
      "/events?type=UPLINK&created_after=" + todayStart.toISOString()
    );
    const todayMessages = todayEventsData.events || [];

    // 更新统计显示
    document.getElementById("total-devices").textContent = allDevices.length;
    document.getElementById("active-devices").textContent =
      activeDeviceEUIs.size;
    document.getElementById("total-gateways").textContent = gateways.length;
    document.getElementById("messages-today").textContent =
      todayMessages.length.toLocaleString();
  } catch (error) {
    console.error("Failed to load dashboard stats:", error);
    // 显示默认值
    document.getElementById("total-devices").textContent = "0";
    document.getElementById("active-devices").textContent = "0";
    document.getElementById("total-gateways").textContent = "0";
    document.getElementById("messages-today").textContent = "0";
  }
}

// Applications
function renderApplicationsTable(apps) {
  const tbody = document.getElementById("applications-table");
  if (apps.length > 0) {
    tbody.innerHTML = apps.map(app => `
      <tr>
        <td>${app.id}</td>
        <td>${app.name}</td>
        <td>${app.description || "-"}</td>
        <td>${app.deviceCount || 0}</td>
        <td>${new Date(app.createdAt).toLocaleDateString()}</td>
        <td>
          <button class="btn btn-sm" onclick="viewApplication('${app.id}')">View</button>
          <button class="btn btn-sm btn-danger" onclick="deleteApplication('${app.id}')">Delete</button>
        </td>
      </tr>
    `).join("");
  } else {
    tbody.innerHTML = '<tr><td colspan="6" style="text-align: center;">No applications found</td></tr>';
  }
}

async function loadApplications() {
  try {
    // 先检查缓存
    const cached = simpleCache.get('applications');
    if (cached) {
      applications = cached;
      renderApplicationsTable(applications);
      return;
    }
    const data = await apiRequest("GET", "/applications");
    applications = data.applications || [];
    // 存入缓存
    simpleCache.set('applications', applications);
 
    // 渲染表格（抽取渲染逻辑）
    renderApplicationsTable(applications);
  } catch (error) {
    console.error("Failed to load applications:", error);
  }
}

// Devices
async function loadDevices() {
  try {
    // 获取所有应用
    if (applications.length === 0) {
      const appsData = await apiRequest("GET", "/applications");
      applications = appsData.applications || [];
    }

    if (applications.length === 0) {
      document.getElementById("devices-table").innerHTML =
        '<tr><td colspan="8" style="text-align: center;">请先创建一个应用</td></tr>';
      return;
    }

    // 显示加载中状态
    document.getElementById("devices-table").innerHTML =
      '<tr><td colspan="8" style="text-align: center;">加载设备数据中...</td></tr>';

    // 加载所有应用的设备
    let allDevices = [];
    for (const app of applications) {
      try {
        const data = await apiRequest(
          "GET",
          `/devices?application_id=${app.id}`
        );

        if (data.devices) {
          // 为每个设备添加应用信息
          for (const device of data.devices) {
            device.applicationName = app.name;
            device.applicationId = app.id;

            // 获取设备的最新数据（像History页面那样）
            try {
              const latestData = await apiRequest(
                "GET",
                `/devices/${device.devEUI}/data?limit=1`
              );
              if (latestData.data && latestData.data.length > 0) {
                const latest = latestData.data[0];
                device.lastSeenAt = latest.receivedAt;
                device.lastRSSI = latest.rssi;
                device.lastSNR = latest.snr;
                device.lastFCnt = latest.fCnt;
              }
            } catch (err) {
              console.log(`No data for device ${device.devEUI}`);
            }
          }

          allDevices = allDevices.concat(data.devices);
        }
      } catch (error) {
        console.error(`Failed to load devices for app ${app.id}:`, error);
      }
    }

    const tbody = document.getElementById("devices-table");

    if (allDevices.length > 0) {
      tbody.innerHTML = allDevices
        .map((device) => {
          const deviceStatus = formatDeviceStatus(device);
          const devAddr = formatDevAddr(device.devAddr);

          // 信号强度显示
          const signalInfo =
            device.lastRSSI !== undefined
              ? `<small style="display: block; color: #666; font-size: 0.85em;">📶 ${device.lastRSSI}dBm / ${device.lastSNR}dB</small>`
              : "";

          return `
            <tr>
              <td class="mono">${device.devEUI}</td>
              <td>${device.name}</td>
              <td>${device.applicationName || "-"}</td>
              <td>
                <span class="${deviceStatus.class}">
                  ${device.isDisabled ? "🚫 已禁用" : deviceStatus.text}
                </span>
              </td>
              <td class="mono">${devAddr || "未激活"}</td>
              <td>
                <span title="最新帧: ${device.lastFCnt || "-"} / 设备计数: ${
            device.fCntUp || 0
          }">
                  ↑${device.lastFCnt || device.fCntUp || 0} ↓${
            device.nFCntDown || 0
          }
                </span>
              </td>
              <td>
                ${
                  device.lastSeenAt
                    ? `<div>${new Date(device.lastSeenAt).toLocaleString(
                        "zh-CN"
                      )}</div>${signalInfo}`
                    : '<span style="color: #999; font-style: italic;">从未</span>'
                }
              </td>
              <td>
                <button class="btn btn-sm" onclick="viewDevice('${
                  device.devEUI
                }')">查看</button>
                ${
                  device.lastSeenAt
                    ? `<button class="btn btn-sm btn-secondary" onclick="sendTestDownlink('${device.devEUI}')">测试</button>`
                    : ""
                }
                <button class="btn btn-sm btn-danger" onclick="deleteDevice('${
                  device.devEUI
                }')">删除</button>
              </td>
            </tr>
          `;
        })
        .join("");
    } else {
      tbody.innerHTML =
        '<tr><td colspan="8" style="text-align: center;">暂无设备</td></tr>';
    }
  } catch (error) {
    console.error("Failed to load devices:", error);
    document.getElementById("devices-table").innerHTML =
      '<tr><td colspan="8" style="text-align: center; color: red;">加载失败，请刷新重试</td></tr>';
  }
}

// Gateways
async function loadGateways() {
  try {
    const data = await apiRequest("GET", "/gateways");
    const gateways = data.gateways || [];

    const tbody = document.getElementById("gateways-table");

    if (gateways.length > 0) {
      tbody.innerHTML = gateways
        .map(
          (gateway) => `
                <tr>
                    <td>${gateway.gatewayId}</td>
                    <td>${gateway.name}</td>
                    <td><span class="status-${
                      gateway.lastSeenAt ? "active" : "inactive"
                    }">${gateway.lastSeenAt ? "Online" : "Offline"}</span></td>
                    <td>${
                      gateway.location
                        ? `${gateway.location.latitude.toFixed(
                            4
                          )}, ${gateway.location.longitude.toFixed(4)}`
                        : "-"
                    }</td>
                    <td>${
                      gateway.lastSeenAt
                        ? new Date(gateway.lastSeenAt).toLocaleString()
                        : "Never"
                    }</td>
                    <td>
                        <button class="btn btn-sm" onclick="viewGateway('${
                          gateway.gatewayId
                        }')">View</button>
                        <button class="btn btn-sm btn-danger" onclick="deleteGateway('${
                          gateway.gatewayId
                        }')">Delete</button>
                    </td>
                </tr>
            `
        )
        .join("");
    } else {
      tbody.innerHTML =
        '<tr><td colspan="6" style="text-align: center;">No gateways found</td></tr>';
    }
  } catch (error) {
    console.error("Failed to load gateways:", error);
  }
}

// Events - 简单自动刷新（与仪表盘相同的方式）
async function loadEvents() {
  try {
    // 获取筛选条件
    const type = document.getElementById("event-filter-type").value;
    const level = document.getElementById("event-filter-level").value;
    
    let endpoint = `/events?limit=100`;
    if (type) endpoint += `&type=${type}`;
    if (level) endpoint += `&level=${level}`;
    
    const data = await apiRequest('GET', endpoint);
    const events = data.events || [];
    const tbody = document.getElementById("events-table");
    
    if (events.length > 0) {
      tbody.innerHTML = events
        .map(event => `
          <tr>
            <td>${new Date(event.createdAt).toLocaleString('zh-CN')}</td>
            <td>${event.type}</td>
            <td><span class="status-${event.level.toLowerCase()}">${event.level}</span></td>
            <td>${event.devEUI || event.gatewayId || "-"}</td>
            <td>${event.description}</td>
          </tr>
        `)
        .join("");
    } else {
      tbody.innerHTML = '<tr><td colspan="5" style="text-align: center;">No events found</td></tr>';
    }
  } catch (error) {
    console.error('Failed to load events:', error);
    document.getElementById('events-table').innerHTML = 
      '<tr><td colspan="5" style="text-align: center; color: red;">Failed to load events</td></tr>';
  }
}

// Settings
async function loadSettings() {
  try {
    const user = await apiRequest("GET", "/users/me");
    document.getElementById("profile-email").value = user.email || "";
    document.getElementById("profile-firstname").value = user.firstName || "";
    document.getElementById("profile-lastname").value = user.lastName || "";
  } catch (error) {
    console.error("Failed to load settings:", error);
  }
}

// Users
async function loadUsers() {
  try {
    const data = await apiRequest("GET", "/users");
    const users = data.users || [];
    const tbody = document.getElementById("users-table");

    if (users.length > 0) {
      tbody.innerHTML = users
        .map(
          (user) => `
                <tr>
                    <td>${user.email}</td>
                    <td>${user.firstName || ""} ${user.lastName || ""}</td>
                    <td><span class="badge ${
                      user.isAdmin ? "badge-admin" : "badge-user"
                    }">${user.isAdmin ? "Admin" : "User"}</span></td>
                    <td><span class="status-${
                      user.isActive ? "active" : "inactive"
                    }">${user.isActive ? "Active" : "Inactive"}</span></td>
                    <td>${new Date(user.createdAt).toLocaleDateString()}</td>
                    <td>
                        <button class="btn btn-sm" onclick="editUser('${
                          user.id
                        }')">Edit</button>
                        ${
                          user.email !== localStorage.getItem("user_email")
                            ? `<button class="btn btn-sm btn-danger" onclick="deleteUser('${user.id}')">Delete</button>`
                            : ""
                        }
                    </td>
                </tr>
            `
        )
        .join("");
    } else {
      tbody.innerHTML =
        '<tr><td colspan="6" style="text-align: center;">No users found</td></tr>';
    }
  } catch (error) {
    console.error("Failed to load users:", error);
    document.getElementById("users-table").innerHTML =
      '<tr><td colspan="6" style="text-align: center;">Failed to load users. Please check console for errors.</td></tr>';
  }
}

// Modal functions
function showModal(title, content, modalClass = "") {
  const modal = document.createElement("div");
  modal.className = "modal";
  modal.innerHTML = `
        <div class="modal-content ${modalClass}">
            <span class="close" onclick="closeModal()">&times;</span>
            <h2>${title}</h2>
            ${content}
        </div>
    `;
  document.body.appendChild(modal);
  modal.style.display = "block";
}

function closeModal() {
  const modal = document.querySelector(".modal");
  if (modal) {
    modal.remove();
  }
}

function showNotification(message, type = "info") {
  const notification = document.createElement("div");
  notification.className = `notification ${type}`;
  notification.textContent = message;
  document.body.appendChild(notification);

  setTimeout(() => {
    notification.remove();
  }, 3000);
}

// Add Application Modal
function showAddApplicationModal() {
  showModal(
    "Add Application",
    `
        <form id="add-application-form">
            <div class="form-group">
                <label>Name *</label>
                <input type="text" id="app-name" required>
            </div>
            <div class="form-group">
                <label>Description</label>
                <textarea id="app-description" rows="3"></textarea>
            </div>
            <button type="submit" class="btn btn-primary">Create Application</button>
        </form>
    `
  );

  document
    .getElementById("add-application-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();

      try {
        await apiRequest("POST", "/applications", {
          name: document.getElementById("app-name").value,
          description: document.getElementById("app-description").value,
        });

        closeModal();
        showNotification("Application created successfully", "success");
        loadApplications();
      } catch (error) {
        console.error("Failed to create application:", error);
      }
    });
}

// Add Device Modal
function showAddDeviceModal() {
  if (applications.length === 0) {
    showNotification("Please create an application first", "error");
    return;
  }

  showModal(
    "Add Device",
    `
        <form id="add-device-form">
            <div class="form-group">
                <label>Application *</label>
                <select id="device-app" required>
                    ${applications
                      .map(
                        (app) =>
                          `<option value="${app.id}">${app.name}</option>`
                      )
                      .join("")}
                </select>
            </div>
            <div class="form-group">
                <label>Device Name *</label>
                <input type="text" id="device-name" required>
            </div>
            <div class="form-group">
                <label>Device EUI *</label>
                <input type="text" id="device-eui" pattern="[0-9A-Fa-f]{16}" maxlength="16" required>
                <small>16 hex characters</small>
            </div>
            <div class="form-group">
                <label>Join EUI (App EUI)</label>
                <input type="text" id="device-join-eui" pattern="[0-9A-Fa-f]{16}" maxlength="16">
                <small>16 hex characters (for OTAA)</small>
            </div>
            <div class="form-group">
                <label>Device Profile</label>
                <select id="device-profile">
                    <option value="44444444-4444-4444-4444-444444444444">Default Profile</option>
                </select>
            </div>
            <div class="form-group">
                <label>Description</label>
                <textarea id="device-description" rows="2"></textarea>
            </div>
            <button type="submit" class="btn btn-primary">Create Device</button>
        </form>
    `
  );

  document
    .getElementById("add-device-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();

      try {
        await apiRequest("POST", "/devices", {
          application_id: document.getElementById("device-app").value,
          name: document.getElementById("device-name").value,
          dev_eui: document.getElementById("device-eui").value,
          join_eui:
            document.getElementById("device-join-eui").value || undefined,
          device_profile_id: document.getElementById("device-profile").value,
          description: document.getElementById("device-description").value,
        });

        closeModal();
        showNotification("Device created successfully", "success");
        loadDevices();
      } catch (error) {
        console.error("Failed to create device:", error);
      }
    });
}

// Add Gateway Modal
function showAddGatewayModal() {
  showModal(
    "Add Gateway",
    `
        <form id="add-gateway-form">
            <div class="form-group">
                <label>Gateway ID *</label>
                <input type="text" id="gateway-id" pattern="[0-9A-Fa-f]{16}" maxlength="16" required>
                <small>16 hex characters</small>
            </div>
            <div class="form-group">
                <label>Name *</label>
                <input type="text" id="gateway-name" required>
            </div>
            <div class="form-group">
                <label>Description</label>
                <textarea id="gateway-description" rows="2"></textarea>
            </div>
            <div class="form-group">
                <label>Latitude</label>
                <input type="number" id="gateway-lat" step="0.000001" min="-90" max="90">
            </div>
            <div class="form-group">
                <label>Longitude</label>
                <input type="number" id="gateway-lng" step="0.000001" min="-180" max="180">
            </div>
            <div class="form-group">
                <label>Altitude (meters)</label>
                <input type="number" id="gateway-alt" step="0.1">
            </div>
            <button type="submit" class="btn btn-primary">Create Gateway</button>
        </form>
    `
  );

  document
    .getElementById("add-gateway-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();

      try {
        await apiRequest("POST", "/gateways", {
          gateway_id: document.getElementById("gateway-id").value,
          name: document.getElementById("gateway-name").value,
          description: document.getElementById("gateway-description").value,
          latitude:
            parseFloat(document.getElementById("gateway-lat").value) || 0,
          longitude:
            parseFloat(document.getElementById("gateway-lng").value) || 0,
          altitude:
            parseFloat(document.getElementById("gateway-alt").value) || 0,
        });

        closeModal();
        showNotification("Gateway created successfully", "success");
        loadGateways();
      } catch (error) {
        console.error("Failed to create gateway:", error);
      }
    });
}

// Add User Modal
function showAddUserModal() {
  showModal(
    "Add User",
    `
        <form id="add-user-form">
            <div class="form-group">
                <label>Email *</label>
                <input type="email" id="user-email" required>
            </div>
            <div class="form-group">
                <label>Password *</label>
                <input type="password" id="user-password" minlength="6" required>
                <small>Minimum 6 characters</small>
            </div>
            <div class="form-group">
                <label>First Name</label>
                <input type="text" id="user-firstname">
            </div>
            <div class="form-group">
                <label>Last Name</label>
                <input type="text" id="user-lastname">
            </div>
            <div class="form-group">
                <label>
                    <input type="checkbox" id="user-is-admin">
                    Administrator privileges
                </label>
            </div>
            <button type="submit" class="btn btn-primary">Create User</button>
        </form>
    `
  );

  document
    .getElementById("add-user-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();

      try {
        await apiRequest("POST", "/users", {
          email: document.getElementById("user-email").value,
          password: document.getElementById("user-password").value,
          firstName: document.getElementById("user-firstname").value,
          lastName: document.getElementById("user-lastname").value,
          is_admin: document.getElementById("user-is-admin").checked,
          tenant_id: "11111111-1111-1111-1111-111111111111", // 默认租户
        });

        closeModal();
        showNotification("User created successfully", "success");
        loadUsers();
      } catch (error) {
        console.error("Failed to create user:", error);
      }
    });
}

// Logout
function logout() {
  localStorage.clear();
  window.location.href = "/login.html";
}

// Delete functions
async function deleteApplication(id) {
  if (confirm("Are you sure you want to delete this application?")) {
    try {
      await apiRequest("DELETE", `/applications/${id}`);
      showNotification("Application deleted successfully", "success");
      loadApplications();
    } catch (error) {
      console.error("Failed to delete application:", error);
    }
  }
}

async function deleteDevice(devEUI) {
  if (confirm("Are you sure you want to delete this device?")) {
    try {
      await apiRequest("DELETE", `/devices/${devEUI}`);
      showNotification("Device deleted successfully", "success");
      loadDevices();
    } catch (error) {
      console.error("Failed to delete device:", error);
    }
  }
}

async function deleteGateway(gatewayId) {
  if (confirm("Are you sure you want to delete this gateway?")) {
    try {
      await apiRequest("DELETE", `/gateways/${gatewayId}`);
      showNotification("Gateway deleted successfully", "success");
      loadGateways();
    } catch (error) {
      console.error("Failed to delete gateway:", error);
    }
  }
}

async function deleteUser(userId) {
  if (confirm("Are you sure you want to delete this user?")) {
    try {
      await apiRequest("DELETE", `/users/${userId}`);
      showNotification("User deleted successfully", "success");
      loadUsers();
    } catch (error) {
      console.error("Failed to delete user:", error);
    }
  }
}

// View Application
async function viewApplication(id) {
  try {
    const app = await apiRequest("GET", `/applications/${id}`);
    const devices = await apiRequest("GET", `/devices?application_id=${id}`);

    showModal(
      "Application Details",
      `
        <div class="app-details" data-app-id="${id}">
            <div class="app-header">
                <h3>${app.name}</h3>
                <p>${app.description || "No description"}</p>
            </div>
            
            <div class="app-stats-grid">
                <div class="stat-card">
                    <h4>Total Devices</h4>
                    <p class="stat-number">${
                      devices.devices ? devices.devices.length : 0
                    }</p>
                </div>
                <div class="stat-card">
                    <h4>Active Devices</h4>
                    <p class="stat-number">${
                      devices.devices
                        ? devices.devices.filter((d) => !d.isDisabled).length
                        : 0
                    }</p>
                </div>
                <div class="stat-card">
                    <h4>Messages Today</h4>
                    <p class="stat-number" id="app-messages-today">0</p>
                </div>
                <div class="stat-card">
                    <h4>Created</h4>
                    <p>${new Date(app.createdAt).toLocaleDateString()}</p>
                </div>
            </div>
            
            <div class="app-section">
                <h4>Integration Settings</h4>
                <div class="integration-tabs">
                    <button class="tab-btn active" onclick="showIntegrationTab('http')">HTTP</button>
                    <button class="tab-btn" onclick="showIntegrationTab('mqtt')">MQTT</button>
                </div>
                
                <div id="http-integration" class="integration-content">
                    <form id="http-integration-form">
                        <div class="form-group">
                            <label>Webhook URL</label>
                            <input type="url" id="http-endpoint" placeholder="https://example.com/webhook">
                        </div>
                        <div class="form-group">
                            <label>HTTP Headers (JSON format)</label>
                            <textarea id="http-headers" rows="3" placeholder='{"Authorization": "Bearer token"}'>{}</textarea>
                        </div>
                        <div class="form-group">
                            <label>
                                <input type="checkbox" id="http-enabled">
                                Enable HTTP Integration
                            </label>
                        </div>
                        <button type="submit" class="btn btn-primary">Save HTTP Settings</button>
                    </form>
                </div>
                
                <div id="mqtt-integration" class="integration-content hidden">
                    <form id="mqtt-integration-form">
                        <div class="form-group">
                            <label>MQTT Broker URL</label>
                            <input type="text" id="mqtt-broker" placeholder="mqtt://broker.example.com:1883">
                        </div>
                        <div class="form-group">
                            <label>Username</label>
                            <input type="text" id="mqtt-username">
                        </div>
                        <div class="form-group">
                            <label>Password</label>
                            <input type="password" id="mqtt-password">
                        </div>
                        <div class="form-group">
                            <label>Topic Template</label>
                            <input type="text" id="mqtt-topic" placeholder="application/{app_id}/device/{dev_eui}/up">
                        </div>
                        <div class="form-group">
                            <label>
                                <input type="checkbox" id="mqtt-enabled">
                                Enable MQTT Integration
                            </label>
                        </div>
                        <button type="submit" class="btn btn-primary">Save MQTT Settings</button>
                    </form>
                </div>
            </div>
            
            <div class="app-section">
                <h4>Devices in this Application</h4>
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>DevEUI</th>
                            <th>Name</th>
                            <th>Status</th>
                            <th>Last Seen</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${
                          devices.devices && devices.devices.length > 0
                            ? devices.devices
                                .map(
                                  (device) => `
                                <tr>
                                    <td class="mono">${device.devEUI}</td>
                                    <td>${device.name}</td>
                                    <td><span class="status-${
                                      device.isDisabled ? "inactive" : "active"
                                    }">${
                                    device.isDisabled ? "Disabled" : "Active"
                                  }</span></td>
                                    <td>${
                                      device.lastSeenAt
                                        ? new Date(
                                            device.lastSeenAt
                                          ).toLocaleString()
                                        : "Never"
                                    }</td>
                                </tr>
                            `
                                )
                                .join("")
                            : '<tr><td colspan="4">No devices in this application</td></tr>'
                        }
                    </tbody>
                </table>
            </div>
        </div>
      `,
      "modal-large"
    );

    // 添加样式
    addIntegrationStyles();

    // 加载集成设置
    loadIntegrationSettings(id);

    // 添加表单事件处理器
    setupIntegrationFormHandlers(id);

    // 加载应用今日消息数
    loadApplicationMessageCount(id);
  } catch (error) {
    console.error("Failed to load application details:", error);
    showNotification("Failed to load application details", "error");
  }
}

// Integration related functions
async function loadIntegrationSettings(appId) {
  try {
    const integrations = await apiRequest(
      "GET",
      `/applications/${appId}/integrations`
    );

    // 填充 HTTP 集成配置
    if (integrations.http) {
      document.getElementById("http-endpoint").value =
        integrations.http.endpoint || "";
      document.getElementById("http-headers").value = JSON.stringify(
        integrations.http.headers || {},
        null,
        2
      );
      document.getElementById("http-enabled").checked =
        integrations.http.enabled || false;
    }

    // 填充 MQTT 集成配置
    if (integrations.mqtt) {
      document.getElementById("mqtt-broker").value =
        integrations.mqtt.brokerUrl || "";
      document.getElementById("mqtt-username").value =
        integrations.mqtt.username || "";
      document.getElementById("mqtt-password").value =
        integrations.mqtt.password || "";
      document.getElementById("mqtt-topic").value =
        integrations.mqtt.topicPattern ||
        "application/{app_id}/device/{dev_eui}/up";
      document.getElementById("mqtt-enabled").checked =
        integrations.mqtt.enabled || false;
    }
  } catch (error) {
    console.error("Failed to load integration settings:", error);
    // 如果加载失败，使用默认值
    document.getElementById("http-headers").value = "{}";
    document.getElementById("mqtt-topic").value =
      "application/{app_id}/device/{dev_eui}/up";
  }
}

function setupIntegrationFormHandlers(appId) {
  // HTTP 集成表单提交
  const httpForm = document.getElementById("http-integration-form");
  if (httpForm) {
    httpForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      await saveHTTPIntegration(appId);
    });
  }

  // MQTT 集成表单提交
  const mqttForm = document.getElementById("mqtt-integration-form");
  if (mqttForm) {
    mqttForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      await saveMQTTIntegration(appId);
    });
  }
}

async function saveHTTPIntegration(appId) {
  try {
    const headers = document.getElementById("http-headers").value;
    let parsedHeaders = {};

    // 解析 JSON headers
    if (headers.trim()) {
      try {
        parsedHeaders = JSON.parse(headers);
      } catch (err) {
        showNotification("Invalid JSON format for headers", "error");
        return;
      }
    }

    const data = {
      enabled: document.getElementById("http-enabled").checked,
      endpoint: document.getElementById("http-endpoint").value,
      headers: parsedHeaders,
      timeout: 30, // 默认30秒超时
    };

    // 验证必填字段
    if (data.enabled && !data.endpoint) {
      showNotification(
        "Webhook URL is required when HTTP integration is enabled",
        "error"
      );
      return;
    }

    // 显示保存状态
    const submitBtn = document.querySelector(
      '#http-integration-form button[type="submit"]'
    );
    const originalText = submitBtn.textContent;
    submitBtn.disabled = true;
    submitBtn.textContent = "Saving...";

    await apiRequest("PUT", `/applications/${appId}/integrations/http`, data);
    showNotification("HTTP integration settings saved successfully", "success");

    // 恢复按钮状态
    submitBtn.disabled = false;
    submitBtn.textContent = originalText;
  } catch (error) {
    console.error("Failed to save HTTP integration:", error);
    showNotification("Failed to save HTTP integration settings", "error");

    // 恢复按钮状态
    const submitBtn = document.querySelector(
      '#http-integration-form button[type="submit"]'
    );
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.textContent = "Save HTTP Settings";
    }
  }
}

async function saveMQTTIntegration(appId) {
  try {
    const data = {
      enabled: document.getElementById("mqtt-enabled").checked,
      brokerUrl: document.getElementById("mqtt-broker").value,
      username: document.getElementById("mqtt-username").value,
      password: document.getElementById("mqtt-password").value,
      topicPattern:
        document.getElementById("mqtt-topic").value ||
        "application/{app_id}/device/{dev_eui}/up",
      qos: 0, // 默认 QoS 0
      tls:
        document.getElementById("mqtt-broker").value.startsWith("mqtts://") ||
        document.getElementById("mqtt-broker").value.includes(":8883"),
    };

    // 验证必填字段
    if (data.enabled && !data.brokerUrl) {
      showNotification(
        "MQTT Broker URL is required when MQTT integration is enabled",
        "error"
      );
      return;
    }

    // 显示保存状态
    const submitBtn = document.querySelector(
      '#mqtt-integration-form button[type="submit"]'
    );
    const originalText = submitBtn.textContent;
    submitBtn.disabled = true;
    submitBtn.textContent = "Saving...";

    await apiRequest("PUT", `/applications/${appId}/integrations/mqtt`, data);
    showNotification("MQTT integration settings saved successfully", "success");

    // 恢复按钮状态
    submitBtn.disabled = false;
    submitBtn.textContent = originalText;
  } catch (error) {
    console.error("Failed to save MQTT integration:", error);
    showNotification("Failed to save MQTT integration settings", "error");

    // 恢复按钮状态
    const submitBtn = document.querySelector(
      '#mqtt-integration-form button[type="submit"]'
    );
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.textContent = "Save MQTT Settings";
    }
  }
}

async function testIntegration(appId, type) {
  // 创建测试模态框
  const testModal = document.createElement("div");
  testModal.className = "modal";
  testModal.innerHTML = `
    <div class="modal-content">
        <h3>Testing ${type.toUpperCase()} Integration</h3>
        <div class="test-progress">
            <div class="loading-spinner"></div>
            <p id="test-status">Connecting...</p>
        </div>
    </div>
  `;
  document.body.appendChild(testModal);
  testModal.style.display = "block";

  try {
    const result = await apiRequest(
      "POST",
      `/applications/${appId}/integrations/test`,
      { type }
    );
    document.getElementById("test-status").textContent =
      "Connection successful!";
    document.querySelector(".loading-spinner").style.display = "none";

    setTimeout(() => {
      testModal.remove();
      showNotification(
        `${type.toUpperCase()} integration test successful!`,
        "success"
      );
    }, 1500);
  } catch (error) {
    document.getElementById(
      "test-status"
    ).textContent = `Test failed: ${error.message}`;
    document.querySelector(".loading-spinner").style.display = "none";

    setTimeout(() => {
      testModal.remove();
      showNotification(
        `${type.toUpperCase()} integration test failed: ${error.message}`,
        "error"
      );
    }, 2000);
  }
}

function showIntegrationTab(tab) {
  document.querySelectorAll(".integration-content").forEach((content) => {
    content.classList.add("hidden");
  });
  document.querySelectorAll(".tab-btn").forEach((btn) => {
    btn.classList.remove("active");
  });

  document.getElementById(`${tab}-integration`).classList.remove("hidden");
  event.target.classList.add("active");
}

async function loadApplicationMessageCount(appId) {
  try {
    const today = new Date().toISOString().split("T")[0];
    const events = await apiRequest(
      "GET",
      `/events?application_id=${appId}&type=UPLINK&created_after=${today}`
    );
    document.getElementById("app-messages-today").textContent = events.events
      ? events.events.length
      : 0;
  } catch (error) {
    console.error("Failed to load message count:", error);
  }
}

function addIntegrationStyles() {
  // 检查是否已经添加过样式
  if (document.getElementById("integration-styles")) {
    return;
  }

  const style = document.createElement("style");
  style.id = "integration-styles";
  style.textContent = `
    .integration-tabs {
        display: flex;
        border-bottom: 1px solid #ddd;
        margin-bottom: 20px;
    }
    
    .tab-btn {
        background: none;
        border: none;
        padding: 10px 20px;
        cursor: pointer;
        border-bottom: 2px solid transparent;
        font-weight: 500;
    }
    
    .tab-btn.active {
        border-bottom-color: #007bff;
        color: #007bff;
    }
    
    .integration-content {
        margin-bottom: 20px;
    }
    
    .integration-content.hidden {
        display: none;
    }
    
    .test-btn {
        background-color: #28a745;
        color: white;
        border: none;
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
    }
    
    .test-btn:hover {
        background-color: #218838;
    }
    
    .test-progress {
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 20px;
    }
    
    .loading-spinner {
        border: 4px solid #f3f3f3;
        border-top: 4px solid #3498db;
        border-radius: 50%;
        width: 30px;
        height: 30px;
        animation: spin 1s linear infinite;
        margin-bottom: 15px;
    }
    
    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
    
    #test-status {
        margin: 0;
        color: #666;
    }
    
    .app-section {
        margin-top: 30px;
        padding-top: 20px;
        border-top: 1px solid #eee;
    }
    
    .app-section h4 {
        margin-bottom: 15px;
        color: #333;
    }
  `;
  document.head.appendChild(style);
}

// View Device
async function viewDevice(devEUI) {
  try {
    // 获取设备详情
    const device = await apiRequest("GET", `/devices/${devEUI}`);

    // 创建设备详情模态框
    showModal(
      "Device Details",
      `
            <div class="modal-large">
                <div class="tabs">
                    <button class="tab-button active" onclick="showDeviceTab('info', '${devEUI}')">Device Info</button>
                    <button class="tab-button" onclick="showDeviceTab('keys', '${devEUI}')">Keys</button>
                    <button class="tab-button" onclick="showDeviceTab('data', '${devEUI}')">Live Data</button>
                    <button class="tab-button" onclick="showDeviceTab('history', '${devEUI}')">History</button>
                    <button class="tab-button" onclick="showDeviceTab('downlink', '${devEUI}')">Downlink</button>
                </div>
                
                <!-- Device Info Tab -->
                <div id="device-info-tab" class="tab-content active">
                    <div class="device-info">
                        <h3>Basic Information</h3>
                        <div class="info-grid">
                            <div class="info-item">
                                <label>Device Name:</label>
                                <span>${device.name}</span>
                            </div>
                            <div class="info-item">
                                <label>Device EUI:</label>
                                <span class="mono">${device.devEUI}</span>
                            </div>
                            <div class="info-item">
                                <label>Application:</label>
                                <span>${
                                  applications.find(
                                    (a) => a.id === device.applicationId
                                  )?.name || "-"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>Status:</label>
                                <span class="status-${
                                  device.isDisabled ? "inactive" : "active"
                                }">
                                    ${device.isDisabled ? "Disabled" : "Active"}
                                </span>
                            </div>
                            <div class="info-item">
                                <label>Join EUI:</label>
                                <span class="mono">${
                                  device.joinEUI || "Not set"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>Device Address:</label>
                                <span class="mono">${
                                  device.devAddr || "Not activated"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>Last Seen:</label>
                                <span>${
                                  device.lastSeenAt
                                    ? new Date(
                                        device.lastSeenAt
                                      ).toLocaleString()
                                    : "Never"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>Battery:</label>
                                <span>${
                                  device.batteryLevel
                                    ? device.batteryLevel + "%"
                                    : "Unknown"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>Frame Counters:</label>
                                <span>Up: ${device.fCntUp || 0} | Down: ${
        device.nFCntDown || 0
      }</span>
                            </div>
                            <div class="info-item">
                                <label>Data Rate:</label>
                                <span>DR${device.dr || 0}</span>
                            </div>
                        </div>
                        <div class="info-item full-width">
                            <label>Description:</label>
                            <p>${device.description || "No description"}</p>
                        </div>
                    </div>
                    <div class="device-actions">
                        <button class="btn btn-secondary" onclick="editDevice('${devEUI}')">Edit Device</button>
                        <button class="btn btn-danger" onclick="deleteDeviceFromModal('${devEUI}')">Delete Device</button>
                    </div>
                </div>
                
                <!-- Keys Tab -->
                <div id="device-keys-tab" class="tab-content">
                    <div class="device-keys">
                        <h3>Device Keys Configuration</h3>
                        <div class="activation-type">
                            <label>Activation Method:</label>
                            <select id="activation-method" onchange="toggleActivationType('${devEUI}')">
                                <option value="OTAA" ${
                                  !device.devAddr ? "selected" : ""
                                }>OTAA (Over-The-Air Activation)</option>
                                <option value="ABP" ${
                                  device.devAddr ? "selected" : ""
                                }>ABP (Activation By Personalization)</option>
                            </select>
                        </div>
                        
                        <!-- OTAA Keys -->
                        <div id="otaa-keys" class="${
                          device.devAddr ? "hidden" : ""
                        }">
                            <h4>OTAA Keys</h4>
                            <form id="otaa-keys-form">
                                <div class="form-group">
                                    <label>App Key (16 bytes hex)</label>
                                    <input type="text" id="device-app-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <div class="form-group">
                                    <label>Network Key (16 bytes hex)</label>
                                    <input type="text" id="device-nwk-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <button type="submit" class="btn btn-primary">Save OTAA Keys</button>
                            </form>
                        </div>
                        
                        <!-- ABP Keys -->
                        <div id="abp-keys" class="${
                          !device.devAddr ? "hidden" : ""
                        }">
                            <h4>ABP Session Keys</h4>
                            <form id="abp-keys-form">
                                <div class="form-group">
                                    <label>Device Address (4 bytes hex)</label>
                                    <input type="text" id="device-dev-addr" pattern="[0-9A-Fa-f]{8}" maxlength="8" value="${
                                      device.devAddr || ""
                                    }">
                                </div>
                                <div class="form-group">
                                    <label>App Session Key (16 bytes hex)</label>
                                    <input type="text" id="device-apps-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <div class="form-group">
                                    <label>Network Session Key (16 bytes hex)</label>
                                    <input type="text" id="device-nwks-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <button type="submit" class="btn btn-primary">Activate Device (ABP)</button>
                            </form>
                        </div>
                    </div>
                </div>
                
                <!-- Live Data Tab -->
                <div id="device-data-tab" class="tab-content">
                    <div class="live-data-section">
                        <h3>Live Data</h3>
                        <div class="data-controls">
                            <button class="btn btn-secondary" onclick="startLiveData('${devEUI}')">Start Live Updates</button>
                            <button class="btn btn-secondary" onclick="stopLiveData()">Stop Updates</button>
                        </div>
                        <div id="live-data-container">
                            <p>Click "Start Live Updates" to begin monitoring device data in real-time.</p>
                        </div>
                    </div>
                </div>
                
                <!-- History Tab -->
                <div id="device-history-tab" class="tab-content">
                    <div class="history-section">
                        <h3>Data History</h3>
                        <div class="history-controls">
                            <select id="history-limit">
                                <option value="20">Last 20 messages</option>
                                <option value="50">Last 50 messages</option>
                                <option value="100">Last 100 messages</option>
                            </select>
                            <button class="btn btn-secondary" onclick="loadDeviceHistory('${devEUI}')">Refresh</button>
                            <button class="btn btn-secondary" onclick="exportDeviceData('${devEUI}')">Export CSV</button>
                        </div>
                        <table class="data-table">
                            <thead>
                                <tr>
                                    <th>Time</th>
                                    <th>FCnt</th>
                                    <th>Port</th>
                                    <th>Data (Hex)</th>
                                    <th>RSSI</th>
                                    <th>SNR</th>
                                    <th>DR</th>
                                </tr>
                            </thead>
                            <tbody id="device-history-table">
                                <tr><td colspan="7">Click "Refresh" to load history</td></tr>
                            </tbody>
                        </table>
                    </div>
                </div>
                
                <!-- Downlink Tab -->
                <div id="device-downlink-tab" class="tab-content">
                    <div class="downlink-section">
                        <h3>Send Downlink Data</h3>
                        <form id="downlink-form">
                            <div class="form-group">
                                <label>FPort (1-223)</label>
                                <input type="number" id="downlink-fport" min="1" max="223" value="1" required>
                            </div>
                            <div class="form-group">
                                <label>Payload (Hex)</label>
                                <input type="text" id="downlink-payload" pattern="[0-9A-Fa-f]*" placeholder="e.g., 0102AABB" required>
                                <small>Enter hex string (max 242 bytes)</small>
                            </div>
                            <div class="form-group">
                                <label>
                                    <input type="checkbox" id="downlink-confirmed">
                                    Confirmed downlink (requires ACK)
                                </label>
                            </div>
                            <button type="submit" class="btn btn-primary">Send Downlink</button>
                        </form>
                        
                        <h4>Pending Downlinks</h4>
                        <div id="pending-downlinks">
                            <p>Loading...</p>
                        </div>
                    </div>
                </div>
            </div>
        `,
      "modal-large"
    );

    // 初始化表单事件
    initializeDeviceModalEvents(devEUI);

    // 加载初始数据
    loadDeviceKeys(devEUI);
    loadPendingDownlinks(devEUI);
  } catch (error) {
    console.error("Failed to load device details:", error);
    showNotification("Failed to load device details", "error");
  }
}

// Device related functions
function showDeviceTab(tab, devEUI) {
  // 隐藏所有标签内容
  document.querySelectorAll(".tab-content").forEach((content) => {
    content.classList.remove("active");
  });

  // 移除所有标签按钮的激活状态
  document.querySelectorAll(".tab-button").forEach((button) => {
    button.classList.remove("active");
  });

  // 显示选中的标签
  document.getElementById(`device-${tab}-tab`).classList.add("active");
  event.target.classList.add("active");

  // 加载标签数据
  if (tab === "history") {
    loadDeviceHistory(devEUI);
  }
}

function toggleActivationType(devEUI) {
  const method = document.getElementById("activation-method").value;
  document
    .getElementById("otaa-keys")
    .classList.toggle("hidden", method !== "OTAA");
  document
    .getElementById("abp-keys")
    .classList.toggle("hidden", method !== "ABP");
}

function initializeDeviceModalEvents(devEUI) {
  // OTAA Keys 表单
  document
    .getElementById("otaa-keys-form")
    ?.addEventListener("submit", async (e) => {
      e.preventDefault();
      await saveOTAAKeys(devEUI);
    });

  // ABP Keys 表单
  document
    .getElementById("abp-keys-form")
    ?.addEventListener("submit", async (e) => {
      e.preventDefault();
      await activateDeviceABP(devEUI);
    });

  // Downlink 表单
  document
    .getElementById("downlink-form")
    ?.addEventListener("submit", async (e) => {
      e.preventDefault();
      await sendDownlink(devEUI);
    });
}

async function loadDeviceKeys(devEUI) {
  try {
    const keys = await apiRequest("GET", `/devices/${devEUI}/keys`);
    if (keys) {
      document.getElementById("device-app-key").value = keys.appKey || "";
      document.getElementById("device-nwk-key").value = keys.nwkKey || "";
    }
  } catch (error) {
    console.log("No keys found for device");
  }
}

async function saveOTAAKeys(devEUI) {
  try {
    const appKey = document.getElementById("device-app-key").value;
    const nwkKey = document.getElementById("device-nwk-key").value;

    await apiRequest("POST", `/devices/${devEUI}/keys`, {
      app_key: appKey,
      nwk_key: nwkKey || appKey, // 如果没有设置 nwk_key，使用 app_key
    });

    showNotification("OTAA keys saved successfully", "success");
  } catch (error) {
    console.error("Failed to save OTAA keys:", error);
  }
}

async function activateDeviceABP(devEUI) {
  try {
    const devAddr = document.getElementById("device-dev-addr").value;
    const appSKey = document.getElementById("device-apps-key").value;
    const nwkSKey = document.getElementById("device-nwks-key").value;

    await apiRequest("POST", `/devices/${devEUI}/activate`, {
      dev_addr: devAddr,
      app_s_key: appSKey,
      nwk_s_key: nwkSKey,
    });

    showNotification("Device activated successfully (ABP)", "success");
    closeModal();
    loadDevices(); // 刷新设备列表
  } catch (error) {
    console.error("Failed to activate device:", error);
  }
}

async function loadDeviceHistory(devEUI) {
  try {
    const limit = document.getElementById("history-limit").value;
    const data = await apiRequest(
      "GET",
      `/devices/${devEUI}/data?limit=${limit}`
    );
    const tbody = document.getElementById("device-history-table");

    if (data.data && data.data.length > 0) {
      tbody.innerHTML = data.data
        .map(
          (frame) => `
                <tr>
                    <td>${new Date(frame.receivedAt).toLocaleString()}</td>
                    <td>${frame.fCnt}</td>
                    <td>${frame.fPort || "-"}</td>
                    <td class="mono">${frame.data || "-"}</td>
                    <td>${frame.rssi} dBm</td>
                    <td>${frame.snr} dB</td>
                    <td>DR${frame.dr}</td>
                </tr>
            `
        )
        .join("");
    } else {
      tbody.innerHTML = '<tr><td colspan="7">No data available</td></tr>';
    }
  } catch (error) {
    console.error("Failed to load device history:", error);
  }
}

async function exportDeviceData(devEUI) {
  try {
    const format = "csv"; // 可以扩展支持其他格式
    window.open(
      `${API_BASE}/devices/${devEUI}/export?format=${format}`,
      "_blank"
    );
    showNotification("Export started", "success");
  } catch (error) {
    console.error("Failed to export device data:", error);
  }
}

async function sendDownlink(devEUI) {
  try {
    const fPort = parseInt(document.getElementById("downlink-fport").value);
    const payload = document.getElementById("downlink-payload").value;
    const confirmed = document.getElementById("downlink-confirmed").checked;

    await apiRequest("POST", `/devices/${devEUI}/downlink`, {
      fPort: fPort,
      data: payload,
      confirmed: confirmed,
    });

    showNotification("Downlink queued successfully", "success");

    // 清空表单
    document.getElementById("downlink-payload").value = "";
    document.getElementById("downlink-confirmed").checked = false;

    // 刷新待发送列表
    loadPendingDownlinks(devEUI);
  } catch (error) {
    console.error("Failed to send downlink:", error);
  }
}

async function loadPendingDownlinks(devEUI) {
  try {
    const data = await apiRequest("GET", `/devices/${devEUI}/downlink`);
    const container = document.getElementById("pending-downlinks");

    if (data.downlinks && data.downlinks.length > 0) {
      container.innerHTML = `
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>Created</th>
                            <th>FPort</th>
                            <th>Data</th>
                            <th>Confirmed</th>
                            <th>Status</th>
                            <th>Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${data.downlinks
                          .map(
                            (dl) => `
                            <tr>
                                <td>${new Date(
                                  dl.createdAt
                                ).toLocaleString()}</td>
                                <td>${dl.fPort}</td>
                                <td class="mono">${dl.data}</td>
                                <td>${dl.confirmed ? "Yes" : "No"}</td>
                                <td>${dl.isPending ? "Pending" : "Sent"}</td>
                                <td>
                                    ${
                                      dl.isPending
                                        ? `<button class="btn btn-sm btn-danger" onclick="cancelDownlink('${dl.id}', '${devEUI}')">Cancel</button>`
                                        : "-"
                                    }
                                </td>
                            </tr>
                        `
                          )
                          .join("")}
                    </tbody>
                </table>
            `;
    } else {
      container.innerHTML = "<p>No pending downlinks</p>";
    }
  } catch (error) {
    console.error("Failed to load pending downlinks:", error);
  }
}

async function cancelDownlink(downlinkId, devEUI) {
  try {
    await apiRequest("DELETE", `/downlinks/${downlinkId}`);
    showNotification("Downlink cancelled", "success");
    loadPendingDownlinks(devEUI);
  } catch (error) {
    console.error("Failed to cancel downlink:", error);
  }
}

// Live data functions
let liveDataInterval = null;

function startLiveData(devEUI) {
  stopLiveData(); // 先停止之前的更新

  const container = document.getElementById("live-data-container");
  container.innerHTML =
    '<p>Monitoring live data...</p><div id="live-data-content"></div>';

  // 立即加载一次
  loadLiveData(devEUI);

  // 每5秒更新一次
  liveDataInterval = setInterval(() => {
    loadLiveData(devEUI);
  }, 5000);

  showNotification("Live updates started", "success");
}

function stopLiveData() {
  if (liveDataInterval) {
    clearInterval(liveDataInterval);
    liveDataInterval = null;
    showNotification("Live updates stopped", "info");
  }
}

async function loadLiveData(devEUI) {
  try {
    const data = await apiRequest("GET", `/devices/${devEUI}/data?limit=1`);
    const content = document.getElementById("live-data-content");

    if (data.data && data.data.length > 0) {
      const latest = data.data[0];
      content.innerHTML = `
                <div class="live-data-display">
                    <div class="data-item">
                        <label>Last Update:</label>
                        <span>${new Date(
                          latest.receivedAt
                        ).toLocaleString()}</span>
                    </div>
                    <div class="data-item">
                        <label>Frame Counter:</label>
                        <span>${latest.fCnt}</span>
                    </div>
                    <div class="data-item">
                        <label>Port:</label>
                        <span>${latest.fPort || "-"}</span>
                    </div>
                    <div class="data-item">
                        <label>Data (Hex):</label>
                        <span class="mono">${latest.data || "No payload"}</span>
                    </div>
                    <div class="data-item">
                        <label>Signal:</label>
                        <span>RSSI: ${latest.rssi} dBm, SNR: ${
        latest.snr
      } dB</span>
                    </div>
                </div>
            `;
    } else {
      content.innerHTML = "<p>No data received yet</p>";
    }
  } catch (error) {
    console.error("Failed to load live data:", error);
  }
}

async function editDevice(devEUI) {
  try {
    const device = await apiRequest("GET", `/devices/${devEUI}`);

    showModal(
      "Edit Device",
      `
            <form id="edit-device-form">
                <div class="form-group">
                    <label>Device Name *</label>
                    <input type="text" id="edit-device-name" value="${
                      device.name
                    }" required>
                </div>
                <div class="form-group">
                    <label>Description</label>
                    <textarea id="edit-device-description" rows="3">${
                      device.description || ""
                    }</textarea>
                </div>
                <div class="form-group">
                    <label>Device Profile</label>
                    <select id="edit-device-profile">
                        <option value="44444444-4444-4444-4444-444444444444" ${
                          device.deviceProfileId ===
                          "44444444-4444-4444-4444-444444444444"
                            ? "selected"
                            : ""
                        }>Default Profile</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="edit-device-disabled" ${
                          device.isDisabled ? "checked" : ""
                        }>
                        Disable device
                    </label>
                </div>
                <button type="submit" class="btn btn-primary">Save Changes</button>
            </form>
        `
    );

    document
      .getElementById("edit-device-form")
      .addEventListener("submit", async (e) => {
        e.preventDefault();

        try {
          await apiRequest("PUT", `/devices/${devEUI}`, {
            name: document.getElementById("edit-device-name").value,
            description: document.getElementById("edit-device-description")
              .value,
            device_profile_id: document.getElementById("edit-device-profile")
              .value,
            is_disabled: document.getElementById("edit-device-disabled")
              .checked,
          });

          closeModal();
          showNotification("Device updated successfully", "success");
          loadDevices();
        } catch (error) {
          console.error("Failed to update device:", error);
        }
      });
  } catch (error) {
    console.error("Failed to load device for editing:", error);
    showNotification("Failed to load device", "error");
  }
}

async function deleteDeviceFromModal(devEUI) {
  if (confirm("Are you sure you want to delete this device?")) {
    try {
      await apiRequest("DELETE", `/devices/${devEUI}`);
      showNotification("Device deleted successfully", "success");
      closeModal();
      loadDevices();
    } catch (error) {
      console.error("Failed to delete device:", error);
    }
  }
}

// View Gateway
async function viewGateway(gatewayId) {
  try {
    const gateway = await apiRequest("GET", `/gateways/${gatewayId}`);

    showModal(
      "Gateway Details",
      `
            <div class="gateway-details">
                <div class="gateway-header">
                    <h3>${gateway.name}</h3>
                    <span class="status-badge ${
                      gateway.lastSeenAt && isOnline(gateway.lastSeenAt)
                        ? "online"
                        : "offline"
                    }">
                        ${
                          gateway.lastSeenAt && isOnline(gateway.lastSeenAt)
                            ? "Online"
                            : "Offline"
                        }
                    </span>
                </div>
                
                <div class="gateway-info-grid">
                    <div class="info-section">
                        <h4>Basic Information</h4>
                        <div class="info-item">
                            <label>Gateway ID:</label>
                            <span class="mono">${gateway.gatewayId}</span>
                        </div>
                        <div class="info-item">
                            <label>Model:</label>
                            <span>${gateway.model || "Unknown"}</span>
                        </div>
                        <div class="info-item">
                            <label>Last Seen:</label>
                            <span>${
                              gateway.lastSeenAt
                                ? new Date(gateway.lastSeenAt).toLocaleString()
                                : "Never"
                            }</span>
                        </div>
                        <div class="info-item">
                            <label>Created:</label>
                            <span>${new Date(
                              gateway.createdAt
                            ).toLocaleString()}</span>
                        </div>
                    </div>
                    
                    <div class="info-section">
                        <h4>Location</h4>
                        <div class="info-item">
                            <label>Latitude:</label>
                            <span>${
                              gateway.location
                                ? gateway.location.latitude.toFixed(6)
                                : "Not set"
                            }</span>
                        </div>
                        <div class="info-item">
                            <label>Longitude:</label>
                            <span>${
                              gateway.location
                                ? gateway.location.longitude.toFixed(6)
                                : "Not set"
                            }</span>
                        </div>
                        <div class="info-item">
                            <label>Altitude:</label>
                            <span>${
                              gateway.location
                                ? gateway.location.altitude + " m"
                                : "Not set"
                            }</span>
                        </div>
                        ${
                          gateway.location
                            ? `<div class="map-container" id="gateway-map-${gatewayId}" style="height: 300px; margin-top: 10px;"></div>`
                            : ""
                        }
                    </div>
                </div>
                
                <div class="gateway-stats">
                    <h4>Statistics (Last 24 Hours)</h4>
                    <div class="stats-grid">
                        <div class="stat-card">
                            <h5>Uplink Messages</h5>
                            <p class="stat-number" id="gw-uplink-count">Loading...</p>
                        </div>
                        <div class="stat-card">
                            <h5>Downlink Messages</h5>
                            <p class="stat-number" id="gw-downlink-count">Loading...</p>
                        </div>
                        <div class="stat-card">
                            <h5>Active Devices</h5>
                            <p class="stat-number" id="gw-device-count">Loading...</p>
                        </div>
                        <div class="stat-card">
                            <h5>Average RSSI</h5>
                            <p class="stat-number" id="gw-avg-rssi">Loading...</p>
                        </div>
                    </div>
                </div>
                
                <div class="gateway-config">
                    <h4>Configuration</h4>
                    <form id="gateway-config-form">
                        <div class="form-group">
                            <label>Gateway Name</label>
                            <input type="text" id="gw-name" value="${
                              gateway.name
                            }">
                        </div>
                        <div class="form-group">
                            <label>Description</label>
                            <textarea id="gw-description" rows="3">${
                              gateway.description || ""
                            }</textarea>
                        </div>
                        <button type="submit" class="btn btn-primary">Update Configuration</button>
                    </form>
                </div>
            </div>
        `,
      "modal-large"
    );

    // 加载网关统计
    loadGatewayStats(gatewayId);

    // 初始化配置表单
    document
      .getElementById("gateway-config-form")
      ?.addEventListener("submit", async (e) => {
        e.preventDefault();
        await updateGatewayConfig(gatewayId);
      });
  } catch (error) {
    console.error("Failed to load gateway details:", error);
    showNotification("Failed to load gateway details", "error");
  }
}

function isOnline(lastSeenAt) {
  const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
  return new Date(lastSeenAt) > fiveMinutesAgo;
}

async function loadGatewayStats(gatewayId) {
  try {
    const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();

    // 获取网关相关事件
    const events = await apiRequest(
      "GET",
      `/events?gateway_id=${gatewayId}&created_after=${oneDayAgo}`
    );

    if (events.events) {
      const uplinkCount = events.events.filter(
        (e) => e.type === "UPLINK"
      ).length;
      const downlinkCount = events.events.filter(
        (e) => e.type === "DOWNLINK"
      ).length;

      // 计算平均RSSI
      const rssiValues = events.events
        .filter((e) => e.type === "UPLINK" && e.metadata && e.metadata.rssi)
        .map((e) => e.metadata.rssi);

      const avgRssi =
        rssiValues.length > 0
          ? (rssiValues.reduce((a, b) => a + b, 0) / rssiValues.length).toFixed(
              1
            )
          : "N/A";

      // 统计活跃设备
      const uniqueDevices = new Set(
        events.events.filter((e) => e.devEUI).map((e) => e.devEUI)
      );

      // 更新显示
      document.getElementById("gw-uplink-count").textContent = uplinkCount;
      document.getElementById("gw-downlink-count").textContent = downlinkCount;
      document.getElementById("gw-device-count").textContent =
        uniqueDevices.size;
      document.getElementById("gw-avg-rssi").textContent = avgRssi + " dBm";
    }
  } catch (error) {
    console.error("Failed to load gateway stats:", error);
    document.querySelectorAll('[id^="gw-"]').forEach((el) => {
      el.textContent = "Error";
    });
  }
}

async function updateGatewayConfig(gatewayId) {
  try {
    const data = {
      name: document.getElementById("gw-name").value,
      description: document.getElementById("gw-description").value,
    };

    await apiRequest("PUT", `/gateways/${gatewayId}`, data);
    showNotification("Gateway configuration updated", "success");
    loadGateways(); // 刷新网关列表
  } catch (error) {
    console.error("Failed to update gateway:", error);
  }
}

// Edit User
async function editUser(userId) {
  try {
    const user = await apiRequest("GET", `/users/${userId}`);

    showModal(
      "Edit User",
      `
            <form id="edit-user-form">
                <div class="form-group">
                    <label>Email *</label>
                    <input type="email" id="edit-user-email" value="${
                      user.email
                    }" required>
                </div>
                <div class="form-group">
                    <label>First Name</label>
                    <input type="text" id="edit-user-firstname" value="${
                      user.firstName || ""
                    }">
                </div>
                <div class="form-group">
                    <label>Last Name</label>
                    <input type="text" id="edit-user-lastname" value="${
                      user.lastName || ""
                    }">
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="edit-user-is-active" ${
                          user.isActive ? "checked" : ""
                        }>
                        Active
                    </label>
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="edit-user-is-admin" ${
                          user.isAdmin ? "checked" : ""
                        }>
                        Administrator privileges
                    </label>
                </div>
                <button type="submit" class="btn btn-primary">Save Changes</button>
            </form>
        `
    );

    document
      .getElementById("edit-user-form")
      .addEventListener("submit", async (e) => {
        e.preventDefault();

        try {
          await apiRequest("PUT", `/users/${userId}`, {
            email: document.getElementById("edit-user-email").value,
            firstName: document.getElementById("edit-user-firstname").value,
            lastName: document.getElementById("edit-user-lastname").value,
            is_active: document.getElementById("edit-user-is-active").checked,
            is_admin: document.getElementById("edit-user-is-admin").checked,
          });

          closeModal();
          showNotification("User updated successfully", "success");
          loadUsers();
        } catch (error) {
          console.error("Failed to update user:", error);
        }
      });
  } catch (error) {
    console.error("Failed to load user:", error);
    showNotification("Failed to load user", "error");
  }
}

// Quick activate device
async function quickActivateDevice(devEUI) {
  showModal(
    "快速激活设备 (ABP)",
    `
      <form id="quick-activate-form">
        <div class="form-group">
          <label>设备地址 (4字节十六进制)</label>
          <input type="text" id="quick-dev-addr" pattern="[0-9A-Fa-f]{8}" maxlength="8" 
                 value="${generateRandomDevAddr()}" required>
          <small>例如: CF9CF17B</small>
        </div>
        <div class="form-group">
          <label>应用会话密钥 (16字节十六进制)</label>
          <input type="text" id="quick-apps-key" pattern="[0-9A-Fa-f]{32}" maxlength="32" 
                 value="${generateRandomKey()}" required>
        </div>
        <div class="form-group">
          <label>网络会话密钥 (16字节十六进制)</label>
          <input type="text" id="quick-nwks-key" pattern="[0-9A-Fa-f]{32}" maxlength="32" 
                 value="${generateRandomKey()}" required>
        </div>
        <button type="submit" class="btn btn-primary">激活设备</button>
      </form>
    `
  );

  document
    .getElementById("quick-activate-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();

      try {
        await apiRequest("POST", `/devices/${devEUI}/activate`, {
          dev_addr: document.getElementById("quick-dev-addr").value,
          app_s_key: document.getElementById("quick-apps-key").value,
          nwk_s_key: document.getElementById("quick-nwks-key").value,
        });

        closeModal();
        showNotification("设备激活成功", "success");
        loadDevices();
      } catch (error) {
        console.error("Failed to activate device:", error);
      }
    });
}

function generateRandomDevAddr() {
  const bytes = new Uint8Array(4);
  crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
    .toUpperCase();
}

function generateRandomKey() {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
    .toUpperCase();
}

// Initialize
document.addEventListener("DOMContentLoaded", () => {
  // Set user info
  const userEmail = localStorage.getItem("user_email");
  if (userEmail) {
    document.querySelector(".user-info span").textContent = userEmail;
  }

  // Load dashboard
  loadDashboard();

  // Handle forms
  document
    .getElementById("profile-form")
    ?.addEventListener("submit", async (e) => {
      e.preventDefault();

      try {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = true;
        submitButton.textContent = "Updating...";

        const userData = {
          firstName: document.getElementById("profile-firstname").value,
          lastName: document.getElementById("profile-lastname").value,
        };

        await apiRequest("PUT", "/users/me", userData);
        showNotification("Profile updated successfully", "success");
      } catch (error) {
        console.error("Failed to update profile:", error);
        showNotification("Failed to update profile", "error");
      } finally {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = false;
        submitButton.textContent = "Update Profile";
      }
    });

  document
    .getElementById("password-form")
    ?.addEventListener("submit", async (e) => {
      e.preventDefault();

      const currentPassword = document.getElementById("current-password").value;
      const newPassword = document.getElementById("new-password").value;
      const confirmPassword = document.getElementById("confirm-password").value;

      // 验证密码
      if (newPassword.length < 6) {
        showNotification(
          "Password must be at least 6 characters long",
          "error"
        );
        return;
      }

      if (newPassword !== confirmPassword) {
        showNotification("Passwords do not match", "error");
        return;
      }

      try {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = true;
        submitButton.textContent = "Changing...";

        await apiRequest("POST", "/users/me/password", {
          currentPassword,
          newPassword,
        });

        showNotification("Password changed successfully", "success");

        // 清空表单
        e.target.reset();
      } catch (error) {
        console.error("Failed to change password:", error);
        showNotification(
          "Failed to change password. Please check your current password.",
          "error"
        );
      } finally {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = false;
        submitButton.textContent = "Change Password";
      }
    });
});

// 页面卸载时清理
window.addEventListener('beforeunload', () => {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
  }
  if (liveDataInterval) {
    clearInterval(liveDataInterval);
  }
});