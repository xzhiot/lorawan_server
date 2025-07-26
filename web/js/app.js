// 简单缓存系统
const simpleCache = {
  data: {},
  set(key, value, ttl = 300000) {
    // 5分钟默认
    this.data[key] = {
      value,
      expires: Date.now() + ttl,
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
  },
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
      throw new Error(data.error || "API请求失败");
    }

    return data;
  } catch (error) {
    console.error("API错误:", error);
    showNotification(error.message, "error");
    throw error;
  }
}

// 自动刷新间隔配置
const AUTO_REFRESH_INTERVALS = {
  dashboard: 30000, // 30秒
  events: 10000, // 10秒 - 事件页面更频繁
  gateways: 15000, // 15秒 - 网关状态更新
  devices: 30000, // 30秒 - 设备列表更新
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
    console.log(`设置 ${currentPage} 页面自动刷新，间隔: ${interval / 1000}秒`);

    autoRefreshTimer = setInterval(() => {
      switch (currentPage) {
        case "dashboard":
          loadDashboard();
          break;
        case "events":
          loadEvents();
          break;
        case "gateways":
          loadGateways();
          break;
        case "devices":
          loadDevices();
          break;
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
        '<tr><td colspan="5" style="text-align: center;">暂无最近活动</td></tr>';
    }
  } catch (error) {
    console.error("加载仪表盘失败:", error);
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
        console.error(`加载应用 ${app.id} 的设备失败:`, error);
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
    console.error("加载仪表盘统计失败:", error);
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
    tbody.innerHTML = apps
      .map(
        (app) => `
      <tr>
        <td>${app.id}</td>
        <td>${app.name}</td>
        <td>${app.description || "-"}</td>
        <td>${app.deviceCount || 0}</td>
        <td>${new Date(app.createdAt).toLocaleDateString()}</td>
        <td>
          <button class="btn btn-sm" onclick="viewApplication('${
            app.id
          }')">查看</button>
          <button class="btn btn-sm btn-danger" onclick="deleteApplication('${
            app.id
          }')">删除</button>
        </td>
      </tr>
    `
      )
      .join("");
  } else {
    tbody.innerHTML =
      '<tr><td colspan="6" style="text-align: center;">未找到应用</td></tr>';
  }
}

async function loadApplications() {
  try {
    // 先检查缓存
    const cached = simpleCache.get("applications");
    if (cached) {
      applications = cached;
      renderApplicationsTable(applications);
      return;
    }
    const data = await apiRequest("GET", "/applications");
    applications = data.applications || [];
    // 存入缓存
    simpleCache.set("applications", applications);

    // 渲染表格（抽取渲染逻辑）
    renderApplicationsTable(applications);
  } catch (error) {
    console.error("加载应用失败:", error);
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
              console.log(`设备 ${device.devEUI} 暂无数据`);
            }
          }

          allDevices = allDevices.concat(data.devices);
        }
      } catch (error) {
        console.error(`加载应用 ${app.id} 的设备失败:`, error);
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
    console.error("加载设备失败:", error);
    document.getElementById("devices-table").innerHTML =
      '<tr><td colspan="8" style="text-align: center; color: red;">加载失败，请刷新重试</td></tr>';
  }
}

// 辅助函数：判断网关是否在线（5分钟内有活动）
function isOnline(lastSeenAt) {
  const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000);
  return new Date(lastSeenAt) > fiveMinutesAgo;
}

// 辅助函数：获取相对时间描述
function getTimeAgo(timestamp) {
  const now = new Date();
  const past = new Date(timestamp);
  const diffMs = now - past;
  const diffSecs = Math.floor(diffMs / 1000);
  const diffMins = Math.floor(diffSecs / 60);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) {
    return `${diffDays}天前离线`;
  } else if (diffHours > 0) {
    return `${diffHours}小时前离线`;
  } else if (diffMins > 5) {
    return `${diffMins}分钟前离线`;
  } else {
    return "刚刚离线";
  }
}

// Gateways
async function loadGateways() {
  try {
    // 显示加载状态
    const tbody = document.getElementById("gateways-table");
    tbody.innerHTML =
      '<tr><td colspan="6" style="text-align: center;">加载中...</td></tr>';

    // 清除缓存，强制获取最新数据
    const data = await apiRequest("GET", "/gateways?t=" + Date.now());
    const gateways = data.gateways || [];

    if (gateways.length > 0) {
      tbody.innerHTML = gateways
        .map((gateway) => {
          // 使用 isOnline 函数判断网关是否真的在线（5分钟内有活动）
          const online = gateway.lastSeenAt && isOnline(gateway.lastSeenAt);

          return `
                <tr>
                    <td>${gateway.gatewayId}</td>
                    <td>${gateway.name}</td>
                    <td>
                        <span class="status-badge ${
                          online ? "online" : "offline"
                        }">
                            ${online ? "🟢 在线" : "🔴 离线"}
                        </span>
                    </td>
                    <td>${
                      gateway.location
                        ? `${gateway.location.latitude.toFixed(
                            4
                          )}, ${gateway.location.longitude.toFixed(4)}`
                        : "-"
                    }</td>
                    <td>
                        ${
                          gateway.lastSeenAt
                            ? `${new Date(gateway.lastSeenAt).toLocaleString(
                                "zh-CN"
                              )}
                               <small style="display: block; color: #666; font-size: 0.8em;">
                                 ${
                                   online
                                     ? "活跃中"
                                     : getTimeAgo(gateway.lastSeenAt)
                                 }
                               </small>`
                            : '<span style="color: #999;">从未上线</span>'
                        }
                    </td>
                    <td>
                        <button class="btn btn-sm" onclick="viewGateway('${
                          gateway.gatewayId
                        }')">查看</button>
                        <button class="btn btn-sm btn-danger" onclick="deleteGateway('${
                          gateway.gatewayId
                        }')">删除</button>
                    </td>
                </tr>
            `;
        })
        .join("");

      // 显示最后更新时间
      console.log(`网关列表已更新 - ${new Date().toLocaleTimeString("zh-CN")}`);
      const updateTimeElement = document.getElementById("gateways-update-time");
      if (updateTimeElement) {
        updateTimeElement.textContent = `最后更新: ${new Date().toLocaleTimeString(
          "zh-CN"
        )}`;
      }
    } else {
      tbody.innerHTML =
        '<tr><td colspan="6" style="text-align: center;">未找到网关</td></tr>';
    }
  } catch (error) {
    console.error("加载网关失败:", error);
    document.getElementById("gateways-table").innerHTML =
      '<tr><td colspan="6" style="text-align: center; color: red;">加载失败，请点击刷新按钮重试</td></tr>';

    // 更新时间显示，即使失败
    const updateTimeElement = document.getElementById("gateways-update-time");
    if (updateTimeElement) {
      updateTimeElement.textContent = `最后尝试: ${new Date().toLocaleTimeString(
        "zh-CN"
      )} (失败)`;
    }
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

    const data = await apiRequest("GET", endpoint);
    const events = data.events || [];
    const tbody = document.getElementById("events-table");

    if (events.length > 0) {
      tbody.innerHTML = events
        .map(
          (event) => `
          <tr>
            <td>${new Date(event.createdAt).toLocaleString("zh-CN")}</td>
            <td>${event.type}</td>
            <td><span class="status-${event.level.toLowerCase()}">${
            event.level
          }</span></td>
            <td>${event.devEUI || event.gatewayId || "-"}</td>
            <td>${event.description}</td>
          </tr>
        `
        )
        .join("");
    } else {
      tbody.innerHTML =
        '<tr><td colspan="5" style="text-align: center;">未找到事件</td></tr>';
    }
  } catch (error) {
    console.error("加载事件失败:", error);
    document.getElementById("events-table").innerHTML =
      '<tr><td colspan="5" style="text-align: center; color: red;">加载事件失败</td></tr>';
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
    console.error("加载设置失败:", error);
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
                    }">${user.isAdmin ? "管理员" : "用户"}</span></td>
                    <td><span class="status-${
                      user.isActive ? "active" : "inactive"
                    }">${user.isActive ? "活跃" : "未激活"}</span></td>
                    <td>${new Date(user.createdAt).toLocaleDateString()}</td>
                    <td>
                        <button class="btn btn-sm" onclick="editUser('${
                          user.id
                        }')">编辑</button>
                        ${
                          user.email !== localStorage.getItem("user_email")
                            ? `<button class="btn btn-sm btn-danger" onclick="deleteUser('${user.id}')">删除</button>`
                            : ""
                        }
                    </td>
                </tr>
            `
        )
        .join("");
    } else {
      tbody.innerHTML =
        '<tr><td colspan="6" style="text-align: center;">未找到用户</td></tr>';
    }
  } catch (error) {
    console.error("加载用户失败:", error);
    document.getElementById("users-table").innerHTML =
      '<tr><td colspan="6" style="text-align: center;">加载用户失败。请检查控制台错误信息。</td></tr>';
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
    "添加应用",
    `
        <form id="add-application-form">
            <div class="form-group">
                <label>名称 *</label>
                <input type="text" id="app-name" required>
            </div>
            <div class="form-group">
                <label>描述</label>
                <textarea id="app-description" rows="3"></textarea>
            </div>
            <button type="submit" class="btn btn-primary">创建应用</button>
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
        showNotification("应用创建成功", "success");
        loadApplications();
      } catch (error) {
        console.error("创建应用失败:", error);
      }
    });
}

// Add Device Modal
function showAddDeviceModal() {
  if (applications.length === 0) {
    showNotification("请先创建一个应用", "error");
    return;
  }

  showModal(
    "添加设备",
    `
        <form id="add-device-form">
            <div class="form-group">
                <label>应用 *</label>
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
                <label>设备名称 *</label>
                <input type="text" id="device-name" required>
            </div>
            <div class="form-group">
                <label>设备EUI *</label>
                <input type="text" id="device-eui" pattern="[0-9A-Fa-f]{16}" maxlength="16" required>
                <small>16位十六进制字符</small>
            </div>
            <div class="form-group">
                <label>入网EUI (App EUI)</label>
                <input type="text" id="device-join-eui" pattern="[0-9A-Fa-f]{16}" maxlength="16">
                <small>16位十六进制字符 (OTAA需要)</small>
            </div>
            <div class="form-group">
                <label>设备配置</label>
                <select id="device-profile">
                    <option value="44444444-4444-4444-4444-444444444444">默认配置</option>
                </select>
            </div>
            <div class="form-group">
                <label>描述</label>
                <textarea id="device-description" rows="2"></textarea>
            </div>
            <button type="submit" class="btn btn-primary">创建设备</button>
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
        showNotification("设备创建成功", "success");
        loadDevices();
      } catch (error) {
        console.error("创建设备失败:", error);
      }
    });
}

// Add Gateway Modal
function showAddGatewayModal() {
  showModal(
    "添加网关",
    `
        <form id="add-gateway-form">
            <div class="form-group">
                <label>网关ID *</label>
                <input type="text" id="gateway-id" pattern="[0-9A-Fa-f]{16}" maxlength="16" required>
                <small>16位十六进制字符</small>
            </div>
            <div class="form-group">
                <label>名称 *</label>
                <input type="text" id="gateway-name" required>
            </div>
            <div class="form-group">
                <label>描述</label>
                <textarea id="gateway-description" rows="2"></textarea>
            </div>
            <div class="form-group">
                <label>纬度</label>
                <input type="number" id="gateway-lat" step="0.000001" min="-90" max="90">
            </div>
            <div class="form-group">
                <label>经度</label>
                <input type="number" id="gateway-lng" step="0.000001" min="-180" max="180">
            </div>
            <div class="form-group">
                <label>高度 (米)</label>
                <input type="number" id="gateway-alt" step="0.1">
            </div>
            <button type="submit" class="btn btn-primary">创建网关</button>
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
        showNotification("网关创建成功", "success");
        loadGateways();
      } catch (error) {
        console.error("创建网关失败:", error);
      }
    });
}

// Add User Modal
function showAddUserModal() {
  showModal(
    "添加用户",
    `
        <form id="add-user-form">
            <div class="form-group">
                <label>邮箱 *</label>
                <input type="email" id="user-email" required>
            </div>
            <div class="form-group">
                <label>密码 *</label>
                <input type="password" id="user-password" minlength="6" required>
                <small>最少6个字符</small>
            </div>
            <div class="form-group">
                <label>名</label>
                <input type="text" id="user-firstname">
            </div>
            <div class="form-group">
                <label>姓</label>
                <input type="text" id="user-lastname">
            </div>
            <div class="form-group">
                <label>
                    <input type="checkbox" id="user-is-admin">
                    管理员权限
                </label>
            </div>
            <button type="submit" class="btn btn-primary">创建用户</button>
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
        showNotification("用户创建成功", "success");
        loadUsers();
      } catch (error) {
        console.error("创建用户失败:", error);
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
  if (confirm("确定要删除这个应用吗？")) {
    try {
      await apiRequest("DELETE", `/applications/${id}`);
      showNotification("应用删除成功", "success");
      loadApplications();
    } catch (error) {
      console.error("删除应用失败:", error);
    }
  }
}

async function deleteDevice(devEUI) {
  if (confirm("确定要删除这个设备吗？")) {
    try {
      await apiRequest("DELETE", `/devices/${devEUI}`);
      showNotification("设备删除成功", "success");
      loadDevices();
    } catch (error) {
      console.error("删除设备失败:", error);
    }
  }
}

async function deleteGateway(gatewayId) {
  if (confirm("确定要删除这个网关吗？")) {
    try {
      await apiRequest("DELETE", `/gateways/${gatewayId}`);
      showNotification("网关删除成功", "success");
      loadGateways();
    } catch (error) {
      console.error("删除网关失败:", error);
    }
  }
}

async function deleteUser(userId) {
  if (confirm("确定要删除这个用户吗？")) {
    try {
      await apiRequest("DELETE", `/users/${userId}`);
      showNotification("用户删除成功", "success");
      loadUsers();
    } catch (error) {
      console.error("删除用户失败:", error);
    }
  }
}

// View Application
async function viewApplication(id) {
  try {
    const app = await apiRequest("GET", `/applications/${id}`);
    const devices = await apiRequest("GET", `/devices?application_id=${id}`);

    showModal(
      "应用详情",
      `
        <div class="app-details" data-app-id="${id}">
            <div class="app-header">
                <h3>${app.name}</h3>
                <p>${app.description || "暂无描述"}</p>
            </div>
            
            <div class="app-stats-grid">
                <div class="stat-card">
                    <h4>设备总数</h4>
                    <p class="stat-number">${
                      devices.devices ? devices.devices.length : 0
                    }</p>
                </div>
                <div class="stat-card">
                    <h4>活跃设备</h4>
                    <p class="stat-number">${
                      devices.devices
                        ? devices.devices.filter((d) => !d.isDisabled).length
                        : 0
                    }</p>
                </div>
                <div class="stat-card">
                    <h4>今日消息</h4>
                    <p class="stat-number" id="app-messages-today">0</p>
                </div>
                <div class="stat-card">
                    <h4>创建时间</h4>
                    <p>${new Date(app.createdAt).toLocaleDateString()}</p>
                </div>
            </div>
            
            <div class="app-section">
                <h4>集成设置</h4>
                <div class="integration-tabs">
                    <button class="tab-btn active" onclick="showIntegrationTab('http')">HTTP</button>
                    <button class="tab-btn" onclick="showIntegrationTab('mqtt')">MQTT</button>
                </div>
                
                <div id="http-integration" class="integration-content">
                    <form id="http-integration-form">
                        <div class="form-group">
                            <label>Webhook地址</label>
                            <input type="url" id="http-endpoint" placeholder="https://example.com/webhook">
                        </div>
                        <div class="form-group">
                            <label>HTTP头部 (JSON格式)</label>
                            <textarea id="http-headers" rows="3" placeholder='{"Authorization": "Bearer token"}'>{}</textarea>
                        </div>
                        <div class="form-group">
                            <label>
                                <input type="checkbox" id="http-enabled">
                                启用HTTP集成
                            </label>
                        </div>
                        <button type="submit" class="btn btn-primary">保存HTTP设置</button>
                    </form>
                </div>
                
                <div id="mqtt-integration" class="integration-content hidden">
                    <form id="mqtt-integration-form">
                        <div class="form-group">
                            <label>MQTT服务器地址</label>
                            <input type="text" id="mqtt-broker" placeholder="mqtt://broker.example.com:1883">
                        </div>
                        <div class="form-group">
                            <label>用户名</label>
                            <input type="text" id="mqtt-username">
                        </div>
                        <div class="form-group">
                            <label>密码</label>
                            <input type="password" id="mqtt-password">
                        </div>
                        <div class="form-group">
                            <label>主题模板</label>
                            <input type="text" id="mqtt-topic" placeholder="application/{app_id}/device/{dev_eui}/up">
                        </div>
                        <div class="form-group">
                            <label>
                                <input type="checkbox" id="mqtt-enabled">
                                启用MQTT集成
                            </label>
                        </div>
                        <button type="submit" class="btn btn-primary">保存MQTT设置</button>
                    </form>
                </div>
            </div>
            
            <div class="app-section">
                <h4>应用中的设备</h4>
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>设备EUI</th>
                            <th>名称</th>
                            <th>状态</th>
                            <th>最后上线</th>
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
                                    device.isDisabled ? "已禁用" : "活跃"
                                  }</span></td>
                                    <td>${
                                      device.lastSeenAt
                                        ? new Date(
                                            device.lastSeenAt
                                          ).toLocaleString()
                                        : "从未"
                                    }</td>
                                </tr>
                            `
                                )
                                .join("")
                            : '<tr><td colspan="4">此应用中暂无设备</td></tr>'
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
    console.error("加载应用详情失败:", error);
    showNotification("加载应用详情失败", "error");
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
    console.error("加载集成设置失败:", error);
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
        showNotification("HTTP头部的JSON格式无效", "error");
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
      showNotification("启用HTTP集成时必须填写Webhook地址", "error");
      return;
    }

    // 显示保存状态
    const submitBtn = document.querySelector(
      '#http-integration-form button[type="submit"]'
    );
    const originalText = submitBtn.textContent;
    submitBtn.disabled = true;
    submitBtn.textContent = "保存中...";

    await apiRequest("PUT", `/applications/${appId}/integrations/http`, data);
    showNotification("HTTP集成设置保存成功", "success");

    // 恢复按钮状态
    submitBtn.disabled = false;
    submitBtn.textContent = originalText;
  } catch (error) {
    console.error("保存HTTP集成失败:", error);
    showNotification("保存HTTP集成设置失败", "error");

    // 恢复按钮状态
    const submitBtn = document.querySelector(
      '#http-integration-form button[type="submit"]'
    );
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.textContent = "保存HTTP设置";
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
      showNotification("启用MQTT集成时必须填写MQTT服务器地址", "error");
      return;
    }

    // 显示保存状态
    const submitBtn = document.querySelector(
      '#mqtt-integration-form button[type="submit"]'
    );
    const originalText = submitBtn.textContent;
    submitBtn.disabled = true;
    submitBtn.textContent = "保存中...";

    await apiRequest("PUT", `/applications/${appId}/integrations/mqtt`, data);
    showNotification("MQTT集成设置保存成功", "success");

    // 恢复按钮状态
    submitBtn.disabled = false;
    submitBtn.textContent = originalText;
  } catch (error) {
    console.error("保存MQTT集成失败:", error);
    showNotification("保存MQTT集成设置失败", "error");

    // 恢复按钮状态
    const submitBtn = document.querySelector(
      '#mqtt-integration-form button[type="submit"]'
    );
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.textContent = "保存MQTT设置";
    }
  }
}

async function testIntegration(appId, type) {
  // 创建测试模态框
  const testModal = document.createElement("div");
  testModal.className = "modal";
  testModal.innerHTML = `
    <div class="modal-content">
        <h3>测试${type.toUpperCase()}集成</h3>
        <div class="test-progress">
            <div class="loading-spinner"></div>
            <p id="test-status">连接中...</p>
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
    document.getElementById("test-status").textContent = "连接成功！";
    document.querySelector(".loading-spinner").style.display = "none";

    setTimeout(() => {
      testModal.remove();
      showNotification(`${type.toUpperCase()}集成测试成功！`, "success");
    }, 1500);
  } catch (error) {
    document.getElementById(
      "test-status"
    ).textContent = `测试失败: ${error.message}`;
    document.querySelector(".loading-spinner").style.display = "none";

    setTimeout(() => {
      testModal.remove();
      showNotification(
        `${type.toUpperCase()}集成测试失败: ${error.message}`,
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
    console.error("加载消息计数失败:", error);
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
      "设备详情",
      `
            <div class="modal-large">
                <div class="tabs">
                    <button class="tab-button active" onclick="showDeviceTab('info', '${devEUI}')">设备信息</button>
                    <button class="tab-button" onclick="showDeviceTab('keys', '${devEUI}')">密钥配置</button>
                    <button class="tab-button" onclick="showDeviceTab('data', '${devEUI}')">实时数据</button>
                    <button class="tab-button" onclick="showDeviceTab('history', '${devEUI}')">历史记录</button>
                    <button class="tab-button" onclick="showDeviceTab('downlink', '${devEUI}')">下行数据</button>
                </div>
                
                <!-- Device Info Tab -->
                <div id="device-info-tab" class="tab-content active">
                    <div class="device-info">
                        <h3>基本信息</h3>
                        <div class="info-grid">
                            <div class="info-item">
                                <label>设备名称:</label>
                                <span>${device.name}</span>
                            </div>
                            <div class="info-item">
                                <label>设备EUI:</label>
                                <span class="mono">${device.devEUI}</span>
                            </div>
                            <div class="info-item">
                                <label>所属应用:</label>
                                <span>${
                                  applications.find(
                                    (a) => a.id === device.applicationId
                                  )?.name || "-"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>状态:</label>
                                <span class="status-${
                                  device.isDisabled ? "inactive" : "active"
                                }">
                                    ${device.isDisabled ? "已禁用" : "活跃"}
                                </span>
                            </div>
                            <div class="info-item">
                                <label>入网EUI:</label>
                                <span class="mono">${
                                  device.joinEUI || "未设置"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>设备地址:</label>
                                <span class="mono">${
                                  device.devAddr || "未激活"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>最后上线:</label>
                                <span>${
                                  device.lastSeenAt
                                    ? new Date(
                                        device.lastSeenAt
                                      ).toLocaleString()
                                    : "从未"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>电池电量:</label>
                                <span>${
                                  device.batteryLevel
                                    ? device.batteryLevel + "%"
                                    : "未知"
                                }</span>
                            </div>
                            <div class="info-item">
                                <label>帧计数器:</label>
                                <span>上行: ${device.fCntUp || 0} | 下行: ${
        device.nFCntDown+1 || 0
      }</span>
                            </div>
                            <div class="info-item">
                                <label>数据速率:</label>
                                <span>DR${device.dr || 0}</span>
                            </div>
                        </div>
                        <div class="info-item full-width">
                            <label>描述:</label>
                            <p>${device.description || "暂无描述"}</p>
                        </div>
                    </div>
                    <div class="device-actions">
                        <button class="btn btn-secondary" onclick="editDevice('${devEUI}')">编辑设备</button>
                        <button class="btn btn-danger" onclick="deleteDeviceFromModal('${devEUI}')">删除设备</button>
                    </div>
                </div>
                
                <!-- Keys Tab -->
                <div id="device-keys-tab" class="tab-content">
                    <div class="device-keys">
                        <h3>设备密钥配置</h3>
                        <div class="activation-type">
                            <label>激活方式:</label>
                            <select id="activation-method" onchange="toggleActivationType('${devEUI}')">
                                <option value="OTAA" ${
                                  !device.devAddr ? "selected" : ""
                                }>OTAA (空中激活)</option>
                                <option value="ABP" ${
                                  device.devAddr ? "selected" : ""
                                }>ABP (个性化激活)</option>
                            </select>
                        </div>
                        
                        <!-- OTAA Keys -->
                        <div id="otaa-keys" class="${
                          device.devAddr ? "hidden" : ""
                        }">
                            <h4>OTAA密钥</h4>
                            <form id="otaa-keys-form">
                                <div class="form-group">
                                    <label>应用密钥 (16字节十六进制)</label>
                                    <input type="text" id="device-app-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <div class="form-group">
                                    <label>网络密钥 (16字节十六进制)</label>
                                    <input type="text" id="device-nwk-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <button type="submit" class="btn btn-primary">保存OTAA密钥</button>
                            </form>
                        </div>
                        
                        <!-- ABP Keys -->
                        <div id="abp-keys" class="${
                          !device.devAddr ? "hidden" : ""
                        }">
                            <h4>ABP会话密钥</h4>
                            <form id="abp-keys-form">
                                <div class="form-group">
                                    <label>设备地址 (4字节十六进制)</label>
                                    <input type="text" id="device-dev-addr" pattern="[0-9A-Fa-f]{8}" maxlength="8" value="${
                                      device.devAddr || ""
                                    }">
                                </div>
                                <div class="form-group">
                                    <label>应用会话密钥 (16字节十六进制)</label>
                                    <input type="text" id="device-apps-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <div class="form-group">
                                    <label>网络会话密钥 (16字节十六进制)</label>
                                    <input type="text" id="device-nwks-key" pattern="[0-9A-Fa-f]{32}" maxlength="32">
                                </div>
                                <button type="submit" class="btn btn-primary">激活设备 (ABP)</button>
                            </form>
                        </div>
                    </div>
                </div>
                
                <!-- Live Data Tab -->
                <div id="device-data-tab" class="tab-content">
                    <div class="live-data-section">
                        <h3>实时数据</h3>
                        <div class="data-controls">
                            <button class="btn btn-secondary" onclick="startLiveData('${devEUI}')">开始实时更新</button>
                            <button class="btn btn-secondary" onclick="stopLiveData()">停止更新</button>
                        </div>
                        <div id="live-data-container">
                            <p>点击"开始实时更新"以监控设备数据。</p>
                        </div>
                    </div>
                </div>
                
                <!-- History Tab -->
                <div id="device-history-tab" class="tab-content">
                    <div class="history-section">
                        <h3>数据历史</h3>
                        <div class="history-controls">
                            <select id="history-limit">
                                <option value="20">最近20条消息</option>
                                <option value="50">最近50条消息</option>
                                <option value="100">最近100条消息</option>
                            </select>
                            <button class="btn btn-secondary" onclick="loadDeviceHistory('${devEUI}')">刷新</button>
                            <button class="btn btn-secondary" onclick="exportDeviceData('${devEUI}')">导出CSV</button>
                        </div>
                        <table class="data-table">
                            <thead>
                                <tr>
                                    <th>时间</th>
                                    <th>帧计数</th>
                                    <th>端口</th>
                                    <th>数据 (十六进制)</th>
                                    <th>信号强度</th>
                                    <th>信噪比</th>
                                    <th>数据速率</th>
                                </tr>
                            </thead>
                            <tbody id="device-history-table">
                                <tr><td colspan="7">点击"刷新"加载历史记录</td></tr>
                            </tbody>
                        </table>
                    </div>
                </div>
                
                <!-- Downlink Tab -->
                <div id="device-downlink-tab" class="tab-content">
                    <div class="downlink-section">
                        <h3>发送下行数据</h3>
                        <form id="downlink-form">
                            <div class="form-group">
                                <label>端口 (1-223)</label>
                                <input type="number" id="downlink-fport" min="1" max="223" value="1" required>
                            </div>
                            <div class="form-group">
                                <label>数据载荷 (十六进制)</label>
                                <input type="text" id="downlink-payload" pattern="[0-9A-Fa-f]*" placeholder="例如: 0102AABB" required>
                                <small>输入十六进制字符串 (最大242字节)</small>
                            </div>
                            <div class="form-group">
                                <label>
                                    <input type="checkbox" id="downlink-confirmed">
                                    确认下行 (需要ACK)
                                </label>
                            </div>
                            <button type="submit" class="btn btn-primary">发送下行数据</button>
                        </form>
                        
                        <h4>待发送数据</h4>
                        <div id="pending-downlinks">
                            <p>加载中...</p>
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
    console.error("加载设备详情失败:", error);
    showNotification("加载设备详情失败", "error");
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
    console.log("未找到设备密钥");
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

    showNotification("OTAA密钥保存成功", "success");
  } catch (error) {
    console.error("保存OTAA密钥失败:", error);
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

    showNotification("设备激活成功 (ABP)", "success");
    closeModal();
    loadDevices(); // 刷新设备列表
  } catch (error) {
    console.error("激活设备失败:", error);
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
      tbody.innerHTML = '<tr><td colspan="7">暂无数据</td></tr>';
    }
  } catch (error) {
    console.error("加载设备历史失败:", error);
  }
}

async function exportDeviceData(devEUI) {
  try {
    const format = "csv"; // 可以扩展支持其他格式
    window.open(
      `${API_BASE}/devices/${devEUI}/export?format=${format}`,
      "_blank"
    );
    showNotification("导出开始", "success");
  } catch (error) {
    console.error("导出设备数据失败:", error);
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

    showNotification("下行数据已加入队列", "success");

    // 清空表单
    document.getElementById("downlink-payload").value = "";
    document.getElementById("downlink-confirmed").checked = false;

    // 刷新待发送列表
    loadPendingDownlinks(devEUI);
  } catch (error) {
    console.error("发送下行数据失败:", error);
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
                            <th>创建时间</th>
                            <th>端口</th>
                            <th>数据</th>
                            <th>确认</th>
                            <th>状态</th>
                            <th>操作</th>
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
                                <td>${dl.confirmed ? "是" : "否"}</td>
                                <td>${dl.isPending ? "待发送" : "已发送"}</td>
                                <td>
                                    ${
                                      dl.isPending
                                        ? `<button class="btn btn-sm btn-danger" onclick="cancelDownlink('${dl.id}', '${devEUI}')">取消</button>`
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
      container.innerHTML = "<p>暂无待发送数据</p>";
    }
  } catch (error) {
    console.error("加载待发送数据失败:", error);
  }
}

async function cancelDownlink(downlinkId, devEUI) {
  try {
    await apiRequest("DELETE", `/downlinks/${downlinkId}`);
    showNotification("下行数据已取消", "success");
    loadPendingDownlinks(devEUI);
  } catch (error) {
    console.error("取消下行数据失败:", error);
  }
}

// Live data functions
let liveDataInterval = null;

function startLiveData(devEUI) {
  stopLiveData(); // 先停止之前的更新

  const container = document.getElementById("live-data-container");
  container.innerHTML =
    '<p>正在监控实时数据...</p><div id="live-data-content"></div>';

  // 立即加载一次
  loadLiveData(devEUI);

  // 每5秒更新一次
  liveDataInterval = setInterval(() => {
    loadLiveData(devEUI);
  }, 5000);

  showNotification("实时更新已开始", "success");
}

function stopLiveData() {
  if (liveDataInterval) {
    clearInterval(liveDataInterval);
    liveDataInterval = null;
    showNotification("实时更新已停止", "info");
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
                        <label>最后更新:</label>
                        <span>${new Date(
                          latest.receivedAt
                        ).toLocaleString()}</span>
                    </div>
                    <div class="data-item">
                        <label>帧计数器:</label>
                        <span>${latest.fCnt}</span>
                    </div>
                    <div class="data-item">
                        <label>端口:</label>
                        <span>${latest.fPort || "-"}</span>
                    </div>
                    <div class="data-item">
                        <label>数据 (十六进制):</label>
                        <span class="mono">${latest.data || "无载荷"}</span>
                    </div>
                    <div class="data-item">
                        <label>信号:</label>
                        <span>RSSI: ${latest.rssi} dBm, SNR: ${
        latest.snr
      } dB</span>
                    </div>
                </div>
            `;
    } else {
      content.innerHTML = "<p>暂未收到数据</p>";
    }
  } catch (error) {
    console.error("加载实时数据失败:", error);
  }
}

async function editDevice(devEUI) {
  try {
    const device = await apiRequest("GET", `/devices/${devEUI}`);

    showModal(
      "编辑设备",
      `
            <form id="edit-device-form">
                <div class="form-group">
                    <label>设备名称 *</label>
                    <input type="text" id="edit-device-name" value="${
                      device.name
                    }" required>
                </div>
                <div class="form-group">
                    <label>描述</label>
                    <textarea id="edit-device-description" rows="3">${
                      device.description || ""
                    }</textarea>
                </div>
                <div class="form-group">
                    <label>设备配置</label>
                    <select id="edit-device-profile">
                        <option value="44444444-4444-4444-4444-444444444444" ${
                          device.deviceProfileId ===
                          "44444444-4444-4444-4444-444444444444"
                            ? "selected"
                            : ""
                        }>默认配置</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="edit-device-disabled" ${
                          device.isDisabled ? "checked" : ""
                        }>
                        禁用设备
                    </label>
                </div>
                <button type="submit" class="btn btn-primary">保存更改</button>
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
          showNotification("设备更新成功", "success");
          loadDevices();
        } catch (error) {
          console.error("更新设备失败:", error);
        }
      });
  } catch (error) {
    console.error("加载设备进行编辑失败:", error);
    showNotification("加载设备失败", "error");
  }
}

async function deleteDeviceFromModal(devEUI) {
  if (confirm("确定要删除这个设备吗？")) {
    try {
      await apiRequest("DELETE", `/devices/${devEUI}`);
      showNotification("设备删除成功", "success");
      closeModal();
      loadDevices();
    } catch (error) {
      console.error("删除设备失败:", error);
    }
  }
}

// View Gateway
async function viewGateway(gatewayId) {
  try {
    const gateway = await apiRequest("GET", `/gateways/${gatewayId}`);

    showModal(
      "网关详情",
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
                            ? "在线"
                            : "离线"
                        }
                    </span>
                </div>
                
                <div class="gateway-info-grid">
                    <div class="info-section">
                        <h4>基本信息</h4>
                        <div class="info-item">
                            <label>网关ID:</label>
                            <span class="mono">${gateway.gatewayId}</span>
                        </div>
                        <div class="info-item">
                            <label>型号:</label>
                            <span>${gateway.model || "未知"}</span>
                        </div>
                        <div class="info-item">
                            <label>最后上线:</label>
                            <span>${
                              gateway.lastSeenAt
                                ? new Date(gateway.lastSeenAt).toLocaleString()
                                : "从未"
                            }</span>
                        </div>
                        <div class="info-item">
                            <label>创建时间:</label>
                            <span>${new Date(
                              gateway.createdAt
                            ).toLocaleString()}</span>
                        </div>
                    </div>
                    
                    <div class="info-section">
                        <h4>位置信息</h4>
                        <div class="info-item">
                            <label>纬度:</label>
                            <span>${
                              gateway.location
                                ? gateway.location.latitude.toFixed(6)
                                : "未设置"
                            }</span>
                        </div>
                        <div class="info-item">
                            <label>经度:</label>
                            <span>${
                              gateway.location
                                ? gateway.location.longitude.toFixed(6)
                                : "未设置"
                            }</span>
                        </div>
                        <div class="info-item">
                            <label>高度:</label>
                            <span>${
                              gateway.location
                                ? gateway.location.altitude + " 米"
                                : "未设置"
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
                    <h4>统计信息 (最近24小时)</h4>
                    <div class="stats-grid">
                        <div class="stat-card">
                            <h5>上行消息</h5>
                            <p class="stat-number" id="gw-uplink-count">加载中...</p>
                        </div>
                        <div class="stat-card">
                            <h5>下行消息</h5>
                            <p class="stat-number" id="gw-downlink-count">加载中...</p>
                        </div>
                        <div class="stat-card">
                            <h5>活跃设备</h5>
                            <p class="stat-number" id="gw-device-count">加载中...</p>
                        </div>
                        <div class="stat-card">
                            <h5>平均信号强度</h5>
                            <p class="stat-number" id="gw-avg-rssi">加载中...</p>
                        </div>
                    </div>
                </div>
                
                <div class="gateway-config">
                    <h4>配置</h4>
                    <form id="gateway-config-form">
                        <div class="form-group">
                            <label>网关名称</label>
                            <input type="text" id="gw-name" value="${
                              gateway.name
                            }">
                        </div>
                        <div class="form-group">
                            <label>描述</label>
                            <textarea id="gw-description" rows="3">${
                              gateway.description || ""
                            }</textarea>
                        </div>
                        <button type="submit" class="btn btn-primary">更新配置</button>
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
    console.error("加载网关详情失败:", error);
    showNotification("加载网关详情失败", "error");
  }
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
    console.error("加载网关统计失败:", error);
    document.querySelectorAll('[id^="gw-"]').forEach((el) => {
      el.textContent = "错误";
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
    showNotification("网关配置更新成功", "success");
    loadGateways(); // 刷新网关列表
  } catch (error) {
    console.error("更新网关失败:", error);
  }
}

// Edit User
async function editUser(userId) {
  try {
    const user = await apiRequest("GET", `/users/${userId}`);

    showModal(
      "编辑用户",
      `
            <form id="edit-user-form">
                <div class="form-group">
                    <label>邮箱 *</label>
                    <input type="email" id="edit-user-email" value="${
                      user.email
                    }" required>
                </div>
                <div class="form-group">
                    <label>名</label>
                    <input type="text" id="edit-user-firstname" value="${
                      user.firstName || ""
                    }">
                </div>
                <div class="form-group">
                    <label>姓</label>
                    <input type="text" id="edit-user-lastname" value="${
                      user.lastName || ""
                    }">
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="edit-user-is-active" ${
                          user.isActive ? "checked" : ""
                        }>
                        活跃
                    </label>
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="edit-user-is-admin" ${
                          user.isAdmin ? "checked" : ""
                        }>
                        管理员权限
                    </label>
                </div>
                <button type="submit" class="btn btn-primary">保存更改</button>
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
          showNotification("用户更新成功", "success");
          loadUsers();
        } catch (error) {
          console.error("更新用户失败:", error);
        }
      });
  } catch (error) {
    console.error("加载用户失败:", error);
    showNotification("加载用户失败", "error");
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
        console.error("激活设备失败:", error);
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
        submitButton.textContent = "更新中...";

        const userData = {
          firstName: document.getElementById("profile-firstname").value,
          lastName: document.getElementById("profile-lastname").value,
        };

        await apiRequest("PUT", "/users/me", userData);
        showNotification("资料更新成功", "success");
      } catch (error) {
        console.error("更新资料失败:", error);
        showNotification("更新资料失败", "error");
      } finally {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = false;
        submitButton.textContent = "更新资料";
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
        showNotification("密码长度至少为6个字符", "error");
        return;
      }

      if (newPassword !== confirmPassword) {
        showNotification("两次输入的密码不一致", "error");
        return;
      }

      try {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = true;
        submitButton.textContent = "修改中...";

        await apiRequest("POST", "/users/me/password", {
          currentPassword,
          newPassword,
        });

        showNotification("密码修改成功", "success");

        // 清空表单
        e.target.reset();
      } catch (error) {
        console.error("修改密码失败:", error);
        showNotification("修改密码失败。请检查当前密码是否正确。", "error");
      } finally {
        const submitButton = e.target.querySelector('button[type="submit"]');
        submitButton.disabled = false;
        submitButton.textContent = "修改密码";
      }
    });
});

// 页面卸载时清理
window.addEventListener("beforeunload", () => {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
  }
  if (liveDataInterval) {
    clearInterval(liveDataInterval);
  }
});
