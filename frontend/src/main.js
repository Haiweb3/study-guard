import { Events } from "@wailsio/runtime";
import * as AppService from "../bindings/study-guard/appservice.js";

const els = {
    statusCard: document.getElementById("statusCard"),
    setupSection: document.getElementById("setupSection"),
    modeSegmented: document.getElementById("modeSegmented"),
    durationInput: document.getElementById("durationInput"),
    useTimerBtn: document.getElementById("useTimerBtn"),
    manualBtn: document.getElementById("manualBtn"),
    startBtn: document.getElementById("startBtn"),
    chipList: document.getElementById("chipList"),
    blacklistCount: document.getElementById("blacklistCount"),
    pickRunningBtn: document.getElementById("pickRunningBtn"),
    runningList: document.getElementById("runningList"),
    historyList: document.getElementById("historyList"),
    appIcon: document.getElementById("appIcon"),
    subtitle: document.getElementById("subtitle"),
};

let config = { blacklist: [] };
let status = { state: "idle" };
let history = [];
let selectedMode = "kill";
let useTimer = false;
let countdownTimer = null;

async function init() {
    const [cfg, st, hist] = await Promise.all([
        AppService.GetConfig(),
        AppService.GetState(),
        AppService.GetHistory(20),
    ]);
    config = cfg;
    status = st;
    history = hist;
    renderAll();

    Events.On("session:changed", (event) => {
        status = event.data;
        renderAll();
    });
}

function renderAll() {
    renderMode();
    renderStatus();
    renderBlacklist();
    renderHistory();
}

function renderMode() {
    els.modeSegmented.querySelectorAll(".seg-btn").forEach((btn) => {
        btn.classList.toggle("active", btn.dataset.mode === selectedMode);
    });
}

function isToday(dateStr) {
    return new Date(dateStr).toDateString() === new Date().toDateString();
}

function renderStatus() {
    clearInterval(countdownTimer);

    if (status.state === "active") {
        els.setupSection.style.display = "none";
        els.appIcon.textContent = status.mode === "kill" ? "🔒" : "🔕";
        els.subtitle.textContent = status.mode === "kill" ? "强制退出模式中" : "静音模式中";
        renderActiveCard();
        updateRing();
        countdownTimer = setInterval(updateRing, 1000);
    } else {
        els.setupSection.style.display = "";
        els.appIcon.textContent = "📚";
        els.subtitle.textContent = "专注屏蔽工具";
        const todays = history.filter((r) => isToday(r.startedAt));
        const mins = Math.round(todays.reduce((sum, r) => sum + r.durationMin, 0));
        els.statusCard.innerHTML = `
      <div class="idle-stats">
        <div class="stat"><span class="stat-num">${mins}</span><span class="stat-label">今日专注(分)</span></div>
        <div class="stat-divider"></div>
        <div class="stat"><span class="stat-num">${todays.length}</span><span class="stat-label">今日次数</span></div>
      </div>`;
    }
}

function formatDuration(ms) {
    const totalSec = Math.max(0, Math.round(ms / 1000));
    const m = Math.floor(totalSec / 60).toString().padStart(2, "0");
    const s = (totalSec % 60).toString().padStart(2, "0");
    return `${m}:${s}`;
}

function renderActiveCard() {
    els.statusCard.innerHTML = `
    <div class="ring ${status.mode === "kill" ? "kill" : ""}" id="focusRing" style="--progress:0">
      <div class="ring-inner">
        <span class="ring-icon">${status.mode === "kill" ? "🔒" : "🔕"}</span>
        <span class="ring-time" id="ringTime">00:00</span>
        <span class="ring-caption" id="ringCaption"></span>
      </div>
    </div>
    <div class="end-zone" id="endZone">
      <button class="end-btn" id="endBtn">结束学习模式</button>
    </div>
  `;
    document.getElementById("endBtn").addEventListener("click", startEndConfirm);
}

// Runs every second while a session is active. Only touches the ring's own
// elements, never #endZone - otherwise it would stomp the anti-laziness
// confirm countdown before its 5 seconds are up.
function updateRing() {
    const ring = document.getElementById("focusRing");
    if (!ring) return;

    const started = new Date(status.startedAt);
    const now = new Date();
    let progress = 100;
    let timeLabel;
    let caption;

    if (status.endsAt) {
        const end = new Date(status.endsAt);
        const total = end - started;
        const remaining = Math.max(0, end - now);
        progress = total > 0 ? Math.min(100, 100 - (remaining / total) * 100) : 100;
        timeLabel = formatDuration(remaining);
        caption = "剩余";
    } else {
        timeLabel = formatDuration(now - started);
        caption = "已专注";
    }

    ring.style.setProperty("--progress", progress);
    document.getElementById("ringTime").textContent = timeLabel;
    document.getElementById("ringCaption").textContent = caption;
}

function startEndConfirm() {
    const endZone = document.getElementById("endZone");
    let secondsLeft = 5;
    endZone.innerHTML = `<button class="end-btn confirm" id="confirmBtn" disabled>请确认 (${secondsLeft})</button>`;
    const confirmBtn = document.getElementById("confirmBtn");

    const interval = setInterval(() => {
        secondsLeft -= 1;
        if (secondsLeft <= 0) {
            clearInterval(interval);
            confirmBtn.disabled = false;
            confirmBtn.textContent = "确认结束";
        } else {
            confirmBtn.textContent = `请确认 (${secondsLeft})`;
        }
    }, 1000);

    confirmBtn.addEventListener("click", async () => {
        clearInterval(interval);
        confirmBtn.disabled = true;
        confirmBtn.textContent = "结束中...";
        try {
            await AppService.EndSession();
            history = await AppService.GetHistory(20);
            renderAll();
        } catch (err) {
            alert("结束失败: " + err);
        }
    });
}

function escapeHtml(s) {
    return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function renderBlacklist() {
    els.blacklistCount.textContent = `(${config.blacklist.length})`;
    els.chipList.innerHTML =
        config.blacklist
            .map(
                (app, i) => `
      <span class="chip">${escapeHtml(app.name)}<button class="chip-remove" data-index="${i}">×</button></span>
    `
            )
            .join("") || '<span class="empty-hint">暂无，添加一个吧</span>';

    els.chipList.querySelectorAll(".chip-remove").forEach((btn) => {
        btn.addEventListener("click", async () => {
            const idx = Number(btn.dataset.index);
            config.blacklist.splice(idx, 1);
            config = await AppService.SaveBlacklist(config.blacklist);
            renderBlacklist();
        });
    });
}

function renderHistory() {
    if (history.length === 0) {
        els.historyList.innerHTML = '<span class="empty-hint">还没有记录</span>';
        return;
    }
    els.historyList.innerHTML = history
        .slice(0, 10)
        .map((r) => {
            const date = new Date(r.startedAt);
            const dateLabel = `${date.getMonth() + 1}/${date.getDate()} ${date
                .getHours()
                .toString()
                .padStart(2, "0")}:${date.getMinutes().toString().padStart(2, "0")}`;
            const modeIcon = r.mode === "kill" ? "🔒" : "🔕";
            const autoTag = r.autoEnded ? '<span class="tag">自动</span>' : "";
            return `<div class="history-row ${r.mode === "kill" ? "kill" : ""}">
        <span class="history-icon">${modeIcon}</span>
        <span class="history-date">${dateLabel}</span>
        <span class="history-dur">${Math.round(r.durationMin)} 分钟</span>
        ${autoTag}
      </div>`;
        })
        .join("");
}

els.modeSegmented.querySelectorAll(".seg-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
        selectedMode = btn.dataset.mode;
        renderMode();
    });
});

els.useTimerBtn.addEventListener("click", () => {
    useTimer = true;
    els.useTimerBtn.classList.add("active");
    els.manualBtn.classList.remove("active");
    els.durationInput.disabled = false;
    els.durationInput.focus();
});

els.manualBtn.addEventListener("click", () => {
    useTimer = false;
    els.manualBtn.classList.add("active");
    els.useTimerBtn.classList.remove("active");
    els.durationInput.disabled = true;
});

els.startBtn.addEventListener("click", async () => {
    const duration = useTimer ? Number(els.durationInput.value || 0) : 0;
    if (useTimer && !(duration > 0)) {
        alert("请输入有效的分钟数");
        return;
    }
    els.startBtn.disabled = true;
    try {
        status = await AppService.StartSession(selectedMode, duration);
        renderAll();
    } catch (err) {
        alert(String(err));
    } finally {
        els.startBtn.disabled = false;
    }
});

els.pickRunningBtn.addEventListener("click", async () => {
    const isHidden = els.runningList.hidden;
    if (!isHidden) {
        els.runningList.hidden = true;
        return;
    }
    els.runningList.hidden = false;
    els.runningList.innerHTML = '<span class="empty-hint">加载中...</span>';
    els.pickRunningBtn.disabled = true;
    try {
        const apps = await AppService.ListRunningApps();
        renderRunningList(apps);
    } catch (err) {
        els.runningList.innerHTML = `<span class="empty-hint">读取失败: ${escapeHtml(String(err))}</span>`;
    } finally {
        els.pickRunningBtn.disabled = false;
    }
});

function renderRunningList(apps) {
    if (apps.length === 0) {
        els.runningList.innerHTML = '<span class="empty-hint">没有检测到正在运行的应用</span>';
        return;
    }
    const existing = new Set(config.blacklist.map((a) => a.processName));
    els.runningList.innerHTML = apps
        .map((name) => {
            const added = existing.has(name);
            return `<button class="running-item" data-name="${escapeHtml(name)}" ${added ? "disabled" : ""}>${escapeHtml(name)}${added ? " · 已在黑名单" : ""}</button>`;
        })
        .join("");

    els.runningList.querySelectorAll(".running-item:not(:disabled)").forEach((btn) => {
        btn.addEventListener("click", async () => {
            const name = btn.dataset.name;
            config.blacklist.push({ name, processName: name });
            config = await AppService.SaveBlacklist(config.blacklist);
            renderBlacklist();
            renderRunningList(apps);
        });
    });
}


init();
