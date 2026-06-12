/* Generated from app-entry.js for sandboxed plugin iframes. */
(() => {
  // ../plugins/builtin/subscription_hub/web/bridge-client.js
  function createBridgeClient(win, handlers = {}) {
    let requestCounter = 0;
    function nextRequestId(prefix) {
      requestCounter += 1;
      return `${prefix}-${Date.now()}-${requestCounter}`;
    }
    function send(type, payload, requestId) {
      const id = requestId || nextRequestId(type.replaceAll(".", "-"));
      win.parent.postMessage({
        version: "1",
        source: "plugin_management_ui",
        type,
        request_id: id,
        ...payload === void 0 ? {} : { payload }
      }, "*");
      return id;
    }
    function normalizeMessage(raw) {
      const message = raw || {};
      if (message.version !== "1" || message.source !== "management_host") {
        return null;
      }
      if (message.type === "error") {
        return {
          ...message,
          error: normalizeBridgeError(message)
        };
      }
      return message;
    }
    function handleMessage(event) {
      const message = normalizeMessage(event.data);
      if (!message) {
        return;
      }
      if (message.error && handlers.onError) {
        handlers.onError(message);
        return;
      }
      if (handlers.onMessage) {
        handlers.onMessage(message);
      }
    }
    win.addEventListener("message", handleMessage);
    return {
      nextRequestId,
      send,
      destroy() {
        win.removeEventListener("message", handleMessage);
      },
      pageReady() {
        return send("page.ready", void 0, nextRequestId("page-ready"));
      },
      reloadSettings() {
        return send("settings.reload", void 0, nextRequestId("settings-reload"));
      },
      saveSettings(values, requestId) {
        return send("settings.save", { values }, requestId || nextRequestId("settings-save"));
      },
      reloadTargets() {
        return send("protocol.targets.reload", void 0, nextRequestId("protocol-targets"));
      },
      resolveIdentities(items, requestId) {
        return send("protocol.identities.resolve", { items }, requestId || nextRequestId("protocol-identities"));
      },
      resolveBilibiliUser(query, requestId) {
        return send("bilibili.user.resolve", { query }, requestId || nextRequestId("bilibili-user"));
      },
      openRenderTemplate(templateId) {
        return send("render_template.open", { template_id: templateId }, nextRequestId("open-template"));
      }
    };
  }
  function normalizeBridgeError(message) {
    const payload = message && message.payload && typeof message.payload === "object" ? message.payload : {};
    return {
      request_id: message && message.request_id ? message.request_id : "",
      code: typeof payload.code === "string" ? payload.code : "bridge.error",
      message: typeof payload.message === "string" && payload.message.trim() ? payload.message.trim() : "\u64CD\u4F5C\u5931\u8D25",
      details: payload.details
    };
  }

  // ../plugins/builtin/subscription_hub/web/services.js
  var SERVICE_ORDER = ["all", "live", "video", "image_text", "article", "repost"];
  var SERVICE_TYPES = SERVICE_ORDER.filter((service) => service !== "all");
  var SERVICE_LABELS = {
    all: "\u5168\u90E8",
    live: "\u76F4\u64AD",
    video: "\u89C6\u9891",
    image_text: "\u56FE\u6587",
    article: "\u6587\u7AE0",
    repost: "\u8F6C\u53D1"
  };
  function trim(value) {
    return String(value ?? "").trim();
  }
  function unique(values) {
    return [...new Set(values.map(trim).filter(Boolean))];
  }
  function normalizeServices(value) {
    const services = unique(Array.isArray(value) ? value : ["all"]).filter((item) => SERVICE_ORDER.includes(item));
    if (!services.length || services.includes("all")) {
      return ["all"];
    }
    const selected = SERVICE_TYPES.filter((service) => services.includes(service));
    return selected.length === SERVICE_TYPES.length ? ["all"] : selected;
  }
  function serviceCheckboxValues(value) {
    if (Array.isArray(value) && value.length === 0) {
      return /* @__PURE__ */ new Set();
    }
    const services = normalizeServices(value);
    if (services.includes("all")) {
      return new Set(SERVICE_ORDER);
    }
    return new Set(services);
  }
  function hasServiceSelection(value) {
    return !(Array.isArray(value) && value.length === 0);
  }
  function servicesKey(services) {
    return normalizeServices(services).join(",");
  }

  // ../plugins/builtin/subscription_hub/web/subscribers.js
  var numericPattern = /^[0-9]+$/;
  function normalizeSubscriber(value) {
    const id = trim(value && value.id);
    if (!numericPattern.test(id)) {
      return null;
    }
    return {
      id,
      nickname: trim(value.nickname),
      group_nickname: trim(value.group_nickname),
      title: trim(value.title),
      role: trim(value.role),
      role_label: trim(value.role_label),
      avatar_url: trim(value.avatar_url)
    };
  }
  function collectSubscriberAvatars(settings) {
    const avatars = /* @__PURE__ */ new Map();
    for (const subscription of settings.subscriptions || []) {
      for (const subscriber of subscription.subscribers || []) {
        if (subscriber.id && subscriber.avatar_url) {
          avatars.set(subscriber.id, subscriber.avatar_url);
        }
      }
    }
    return avatars;
  }
  function subscriberAvatarURL(avatars, userId) {
    const id = trim(userId);
    return avatars && avatars.get(id) || `https://q1.qlogo.cn/g?b=qq&nk=${encodeURIComponent(id)}&s=640`;
  }
  function identityKey(targetType, targetId, userId) {
    return `${targetType}:${targetId}:${userId}`;
  }
  function buildIdentityRequests(rows) {
    const items = [];
    for (const row of rows || []) {
      for (const target of row.targets || []) {
        for (const userId of row.subscriber_ids || []) {
          items.push({
            target_type: target.target_type,
            target_id: target.target_id,
            user_id: userId
          });
        }
      }
    }
    const seen = /* @__PURE__ */ new Set();
    return items.filter((item) => {
      const key = identityKey(item.target_type, item.target_id, item.user_id);
      if (seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    });
  }

  // ../plugins/builtin/subscription_hub/web/targets.js
  var TARGET_LABELS = {
    group: "\u7FA4\u804A",
    private: "\u79C1\u804A"
  };
  var numericPattern2 = /^[0-9]+$/;
  function targetKey(targetType, targetId) {
    return `${trim(targetType)}:${trim(targetId)}`;
  }
  function deriveTargetAvatarURL(targetType, targetId) {
    const id = trim(targetId);
    if (targetType === "private" && numericPattern2.test(id)) {
      return `https://q1.qlogo.cn/g?b=qq&nk=${encodeURIComponent(id)}&s=640`;
    }
    if (targetType === "group" && numericPattern2.test(id)) {
      return `https://p.qlogo.cn/gh/${encodeURIComponent(id)}/${encodeURIComponent(id)}/100`;
    }
    return "";
  }
  function normalizeTargets(payload) {
    return {
      loaded: true,
      available: payload && payload.available === true,
      groups: Array.isArray(payload && payload.groups) ? payload.groups : [],
      private_users: Array.isArray(payload && payload.private_users) ? payload.private_users : [],
      issues: Array.isArray(payload && payload.issues) ? payload.issues : []
    };
  }
  function allTargets(targetsState) {
    const state2 = targetsState || {};
    return [
      ...(state2.groups || []).map((target) => ({
        key: targetKey("group", target.target_id),
        target_type: "group",
        target_id: trim(target.target_id),
        label: trim(target.target_name) || trim(target.target_id),
        avatar_url: trim(target.avatar_url) || deriveTargetAvatarURL("group", target.target_id)
      })),
      ...(state2.private_users || []).map((target) => ({
        key: targetKey("private", target.target_id),
        target_type: "private",
        target_id: trim(target.target_id),
        label: trim(target.nickname) || trim(target.target_id),
        avatar_url: trim(target.avatar_url) || deriveTargetAvatarURL("private", target.target_id)
      }))
    ];
  }
  function targetMap(targetsState) {
    return new Map(allTargets(targetsState).map((target) => [target.key, target]));
  }
  function currentTargetsForMode(targetsState, mode) {
    return allTargets(targetsState).filter((target) => target.target_type === mode);
  }
  function targetDisplay(target, map) {
    const live = map && map.get(target.key);
    const label = live ? live.label : target.target_name || target.target_id;
    return `${TARGET_LABELS[target.target_type] || target.target_type} ${label}`;
  }
  function targetAvatar(target, map) {
    const live = map && map.get(target.key);
    return live ? live.avatar_url : deriveTargetAvatarURL(target.target_type, target.target_id);
  }

  // ../plugins/builtin/subscription_hub/web/model.js
  function normalizeSubscription(value) {
    if (!value || typeof value !== "object") {
      return null;
    }
    const uid = trim(value.uid);
    const targetType = trim(value.target_type);
    const targetId = trim(value.target_id);
    if (!/^[0-9]+$/.test(uid) || !["group", "private"].includes(targetType) || !/^[0-9]+$/.test(targetId)) {
      return null;
    }
    return {
      id: trim(value.id),
      platform: "bilibili",
      uid,
      name: trim(value.name) || uid,
      avatar_url: trim(value.avatar_url),
      target_type: targetType,
      target_id: targetId,
      target_name: trim(value.target_name),
      services: normalizeServices(value.services),
      subscribers: Array.isArray(value.subscribers) ? value.subscribers.map(normalizeSubscriber).filter(Boolean) : [],
      enabled: value.enabled !== false
    };
  }
  function normalizeSettings(value) {
    const record = value && typeof value === "object" ? value : {};
    return {
      enabled: record.enabled !== false,
      subscriptions: Array.isArray(record.subscriptions) ? record.subscriptions.map(normalizeSubscription).filter(Boolean) : []
    };
  }
  function createBlankRow(rowId) {
    return {
      row_id: rowId,
      uid: "",
      name: "",
      avatar_url: "",
      query: "",
      resolved: false,
      resolve_state: "idle",
      resolve_message: "",
      candidates: [],
      enabled: true,
      services: ["all"],
      service_mode: "common",
      target_mode: "group",
      targets: [],
      subscriber_ids: [],
      edit_mode: true,
      _editSnapshot: null
    };
  }
  function buildRowsFromSettings(settings) {
    const grouped = /* @__PURE__ */ new Map();
    for (const subscription of settings.subscriptions || []) {
      let row = grouped.get(subscription.uid);
      if (!row) {
        row = {
          row_id: `uid-${subscription.uid}`,
          uid: subscription.uid,
          name: subscription.name || subscription.uid,
          avatar_url: subscription.avatar_url || "",
          query: subscription.name || subscription.uid,
          resolved: true,
          resolve_state: "resolved",
          resolve_message: "",
          candidates: [],
          enabled: false,
          services: normalizeServices(subscription.services),
          service_mode: "common",
          target_mode: subscription.target_type || "group",
          targets: [],
          subscriber_ids: [],
          edit_mode: false,
          _editSnapshot: null
        };
        grouped.set(subscription.uid, row);
      }
      row.enabled = row.enabled || subscription.enabled !== false;
      row.avatar_url = row.avatar_url || subscription.avatar_url || "";
      row.name = row.name || subscription.name || subscription.uid;
      row.query = row.name;
      const key = targetKey(subscription.target_type, subscription.target_id);
      row.targets.push({
        key,
        subscription_id: subscription.id,
        target_type: subscription.target_type,
        target_id: subscription.target_id,
        target_name: subscription.target_name || "",
        services: normalizeServices(subscription.services)
      });
      for (const subscriber of subscription.subscribers || []) {
        if (subscriber.id) {
          row.subscriber_ids.push(subscriber.id);
        }
      }
    }
    const rows = [...grouped.values()];
    for (const row of rows) {
      row.subscriber_ids = unique(row.subscriber_ids);
      const serviceKeys = unique(row.targets.map((target) => servicesKey(target.services)));
      if (serviceKeys.length > 1) {
        row.service_mode = "mixed";
      } else if (serviceKeys.length === 1) {
        row.services = row.targets[0].services;
      }
    }
    return rows;
  }
  function cloneRow(row) {
    return JSON.parse(JSON.stringify(row));
  }

  // ../plugins/builtin/subscription_hub/web/settings-payload.js
  function buildSettingsPayload(settings, rows, targetsByKey) {
    const targets = targetsByKey || /* @__PURE__ */ new Map();
    const subscriptions = [];
    for (const row of rows || []) {
      for (const target of row.targets || []) {
        const live = targets.get(target.key);
        const targetName = live ? live.label : target.target_name;
        subscriptions.push({
          id: target.subscription_id || `bilibili-${row.uid}-${target.target_type}-${target.target_id}`,
          platform: "bilibili",
          uid: row.uid,
          name: row.name,
          avatar_url: row.avatar_url,
          target_type: target.target_type,
          target_id: target.target_id,
          target_name: targetName,
          services: normalizeServices(row.service_mode === "mixed" ? target.services : row.services),
          subscribers: (row.subscriber_ids || []).map((userId) => ({ id: trim(userId) })),
          enabled: row.enabled
        });
      }
    }
    return {
      enabled: settings.enabled !== false,
      subscriptions
    };
  }

  // ../plugins/builtin/subscription_hub/web/validation.js
  function validateRow(row, context) {
    const map = context && context.targetMap ? context.targetMap : /* @__PURE__ */ new Map();
    const targetsLoaded = Boolean(context && context.targetsLoaded);
    const errors = [];
    if (!row.resolved || !numericPattern.test(row.uid) || !row.name) {
      errors.push("UP \u672A\u5B8C\u6210\u6821\u9A8C");
    }
    if (!targetsLoaded) {
      errors.push("\u63A8\u9001\u5BF9\u8C61\u672A\u8F7D\u5165");
    }
    if (!row.targets.length) {
      errors.push("\u8BF7\u9009\u62E9\u63A8\u9001\u5BF9\u8C61");
    }
    for (const target of row.targets) {
      if (!map.has(target.key)) {
        errors.push(`${targetDisplay(target, map)} \u4E0D\u5728\u534F\u8BAE\u5BF9\u8C61\u5217\u8868\u4E2D`);
      }
      if (row.service_mode === "mixed" && !hasServiceSelection(target.services)) {
        errors.push(`${targetDisplay(target, map)} \u8BF7\u9009\u62E9\u63A8\u9001\u7C7B\u578B`);
      }
    }
    if (row.service_mode !== "mixed" && !hasServiceSelection(row.services)) {
      errors.push("\u8BF7\u9009\u62E9\u63A8\u9001\u7C7B\u578B");
    }
    for (const id of row.subscriber_ids) {
      if (!numericPattern.test(trim(id))) {
        errors.push(`\u8BA2\u9605\u4EBA QQ \u4E0D\u5408\u6CD5\uFF1A${id}`);
      }
    }
    return unique(errors);
  }
  function validateRows(rows, context) {
    const errors = (rows || []).flatMap((row) => validateRow(row, context));
    return { ok: errors.length === 0, errors };
  }

  // ../plugins/builtin/subscription_hub/web/render/html.js
  function escapeHTML(value) {
    return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
  }
  function selectorValue(value) {
    return String(value ?? "").replaceAll("\\", "\\\\").replaceAll('"', '\\"');
  }
  function generateHueFromString(value) {
    let hash = 0;
    const text = trim(value) || "?";
    for (let i = 0; i < text.length; i += 1) {
      hash = text.charCodeAt(i) + ((hash << 5) - hash);
    }
    return Math.abs(hash) % 360;
  }
  function avatarHTML(avatarUrl, fallbackText, sizeClass, alt) {
    const hue = generateHueFromString(fallbackText || "?");
    const bg = `hsl(${hue} 72% 58%)`;
    const text = (fallbackText || "?").slice(0, 1).toUpperCase();
    const safeBg = escapeHTML(bg);
    const safeText = escapeHTML(text);
    const safeAlt = escapeHTML(alt || "");
    const safeUrl = escapeHTML(avatarUrl || "");
    const safeSize = escapeHTML(sizeClass);
    return `
    <span class="avatar ${safeSize}" style="background:${safeBg}" aria-label="${safeAlt}">
      <img src="${safeUrl}" alt="${safeAlt}" loading="lazy" referrerpolicy="no-referrer" onerror="this.style.display='none'; this.parentNode.querySelector('.avatar-fallback__text').style.display='flex'" />
      <span class="avatar-fallback__text">${safeText}</span>
    </span>
  `;
  }
  function avatarStackHTML(items, maxVisible, sizeClass, getAvatar, getLabel) {
    if (!items.length) {
      return '<span class="sub-card__summary-label">\u65E0</span>';
    }
    const visible = items.slice(0, maxVisible);
    const overflow = items.length - visible.length;
    const avatars = visible.map((item) => avatarHTML(getAvatar(item), getLabel(item), sizeClass, getLabel(item))).join("");
    const overflowHTML = overflow > 0 ? `<span class="avatar-stack__overflow">+${overflow}</span>` : "";
    return `<span class="avatar-stack">${avatars}${overflowHTML}</span>`;
  }

  // ../plugins/builtin/subscription_hub/web/render/layout.js
  function renderEmptyState() {
    return '<div class="empty-state"><p>\u6CA1\u6709\u5339\u914D\u7684\u8BA2\u9605</p><p>\u53EF\u6DFB\u52A0\u8BA2\u9605\u6216\u8C03\u6574\u7B5B\u9009\u6761\u4EF6</p></div>';
  }

  // ../plugins/builtin/subscription_hub/web/render/service-picker.js
  function serviceTagsHTML(services) {
    return serviceCheckboxValues(services).has("all") ? '<span class="service-tag">\u5168\u90E8</span>' : [...serviceCheckboxValues(services)].map((service) => `
      <span class="service-tag">${escapeHTML(SERVICE_LABELS[service] || service)}</span>
    `).join("");
  }
  function renderServiceCheckboxes(rowId, targetKeyValue, services) {
    const active = serviceCheckboxValues(services);
    return SERVICE_ORDER.map((service) => `
    <label>
      <input type="checkbox" class="service-checkbox" data-row-id="${escapeHTML(rowId)}" data-target-key="${escapeHTML(targetKeyValue)}" value="${escapeHTML(service)}" ${active.has(service) ? "checked" : ""} />
      ${escapeHTML(SERVICE_LABELS[service])}
    </label>
  `).join("");
  }
  function renderServiceEditor(row, context) {
    if (row.service_mode === "mixed") {
      return renderMixedServices(row, context.targetMap);
    }
    return `<div class="inline-checks" aria-label="\u63A8\u9001\u7C7B\u578B">${renderServiceCheckboxes(row.row_id, "common", row.services)}</div>`;
  }
  function renderMixedServices(row, map) {
    return `
    <div class="target-service-editor">
      <span class="badge badge--warning">\u76EE\u6807\u914D\u7F6E\u4E0D\u540C</span>
      ${row.targets.map((target) => `
        <div class="target-service-line">
          <span class="row-note">${escapeHTML(targetDisplay(target, map))}</span>
          <div class="inline-checks">${renderServiceCheckboxes(row.row_id, target.key, target.services)}</div>
        </div>
      `).join("")}
    </div>
  `;
  }

  // ../plugins/builtin/subscription_hub/web/render/subscriber-editor.js
  function renderSubscriberChips(row, context) {
    return row.subscriber_ids.length ? row.subscriber_ids.map((id) => `
        <span class="chip">
          ${avatarHTML(subscriberAvatarURL(context.subscriberAvatars, id), `QQ ${id}`, "avatar--candidate", `QQ ${id}`)}
          <span>QQ ${escapeHTML(id)}</span>
          <button type="button" aria-label="\u79FB\u9664\u8BA2\u9605\u4EBA" data-action="remove-subscriber" data-row-id="${escapeHTML(row.row_id)}" data-user-id="${escapeHTML(id)}">\xD7</button>
        </span>
      `).join("") : '<span class="chip chip--success">\u7CFB\u7EDF\u8BA2\u9605</span>';
  }
  function renderSubscriberEditor(row, context) {
    return `
    <div class="subscriber-line">
      <input class="subscriber-input" data-row-id="${escapeHTML(row.row_id)}" type="text" inputmode="numeric" autocomplete="off" placeholder="QQ \u53F7\uFF0C\u7559\u7A7A\u4E3A\u7CFB\u7EDF\u8BA2\u9605" />
      <button type="button" class="button button--small" data-action="add-subscriber" data-row-id="${escapeHTML(row.row_id)}">\u6DFB\u52A0</button>
    </div>
    <div class="chip-list subscriber-chip-list" data-row-id="${escapeHTML(row.row_id)}">${renderSubscriberChips(row, context)}</div>
    <div class="row-note">\u53EA\u4FDD\u5B58 QQ \u53F7\uFF0C\u6635\u79F0\u548C\u7FA4\u540D\u7247\u4FDD\u5B58\u65F6\u5237\u65B0\u3002</div>
  `;
  }

  // ../plugins/builtin/subscription_hub/web/render/status.js
  function renderRowValidation(row, context) {
    const validation = validateRow(row, context);
    return validation.length ? `<ul class="validation-list">${validation.map((item) => `<li>${escapeHTML(item)}</li>`).join("")}</ul>` : '<span class="badge badge--success">\u53EF\u4FDD\u5B58</span>';
  }
  function renderValidationBadge(row, context) {
    return validateRow(row, context).length ? '<span class="badge badge--danger">\u9700\u5904\u7406</span>' : '<span class="badge badge--success">\u53EF\u4FDD\u5B58</span>';
  }

  // ../plugins/builtin/subscription_hub/web/render/target-picker.js
  function renderSelectedTargets(row, context) {
    const map = context.targetMap;
    return row.targets.length ? row.targets.map((target) => `
        <span class="chip ${map.has(target.key) ? "" : "badge--warning"}">
          ${avatarHTML(targetAvatar(target, map), targetDisplay(target, map), "avatar--candidate", targetDisplay(target, map))}
          <span>${escapeHTML(targetDisplay(target, map))}</span>
          <button type="button" aria-label="\u79FB\u9664\u63A8\u9001\u5BF9\u8C61" data-action="remove-target" data-row-id="${escapeHTML(row.row_id)}" data-target-key="${escapeHTML(target.key)}">\xD7</button>
        </span>
      `).join("") : '<span class="chip">\u672A\u9009\u62E9\u63A8\u9001\u5BF9\u8C61</span>';
  }
  function renderTargetOptions(row, context) {
    const selected = new Set(row.targets.map((target) => target.key));
    const targets = currentTargetsForMode(context.targets, row.target_mode);
    if (!targets.length) {
      return '<div class="target-option-empty">\u6CA1\u6709\u53EF\u9009\u5BF9\u8C61</div>';
    }
    return targets.map((target) => {
      const isSelected = selected.has(target.key);
      return `
      <button type="button" class="target-option ${isSelected ? "is-selected" : ""}" data-action="toggle-target" data-row-id="${escapeHTML(row.row_id)}" data-target-key="${escapeHTML(target.key)}" role="option" aria-selected="${isSelected ? "true" : "false"}">
        <span class="target-option__mark" aria-hidden="true">${isSelected ? "\u2713" : ""}</span>
        <span class="target-option__label">${escapeHTML(target.label)}</span>
        <span class="target-option__id">${escapeHTML(target.target_id)}</span>
      </button>
    `;
    }).join("");
  }

  // ../plugins/builtin/subscription_hub/web/render/row-edit.js
  function renderRowEdit(row, context) {
    const title = row.name || row.uid || "\u672A\u6821\u9A8C UP";
    const subtitle = row.uid ? `UID ${row.uid}` : "\u8F93\u5165 UID \u6216 Bilibili \u7528\u6237\u540D\u540E\u6821\u9A8C";
    const upAvatar = avatarHTML(row.avatar_url, title, "avatar--up", title);
    const candidates = row.candidates.length ? `<div class="candidate-list">${row.candidates.map((candidate) => `
        <button type="button" class="button candidate-button" data-action="choose-candidate" data-row-id="${escapeHTML(row.row_id)}" data-user='${escapeHTML(JSON.stringify(candidate))}'>
          ${avatarHTML(candidate.avatar_url, candidate.name, "avatar--candidate", candidate.name)}
          <span>${escapeHTML(candidate.name)} \xB7 UID ${escapeHTML(candidate.uid)}</span>
        </button>
      `).join("")}</div>` : "";
    return `
    <article class="sub-card sub-card--editing ${row.enabled ? "" : "sub-card--disabled"}" data-row-id="${escapeHTML(row.row_id)}">
      <div class="sub-card__head">
        ${upAvatar}
        <div class="sub-card__meta">
          <strong>${escapeHTML(title)}</strong>
          <small>${escapeHTML(subtitle)}</small>
        </div>
        <div class="sub-card__status">
          <span class="badge">${row.resolved ? "\u5DF2\u6821\u9A8C" : row.resolve_state === "checking" ? "\u6821\u9A8C\u4E2D" : "\u5F85\u6821\u9A8C"}</span>
          <div class="row-validation-slot" data-row-id="${escapeHTML(row.row_id)}">${renderRowValidation(row, context)}</div>
        </div>
      </div>

      <div class="sub-card__body">
        <div class="sub-card__section">
          <div class="sub-card__section-title">UP \u4FE1\u606F</div>
          <div class="up-input-line">
            <input class="up-query-input" data-row-id="${escapeHTML(row.row_id)}" type="text" autocomplete="off" value="${escapeHTML(row.query)}" placeholder="UID \u6216 Bilibili \u7528\u6237\u540D" />
            <button type="button" class="button button--small" data-action="resolve-up" data-row-id="${escapeHTML(row.row_id)}">\u6821\u9A8C</button>
          </div>
          ${row.resolve_message ? `<div class="row-note">${escapeHTML(row.resolve_message)}</div>` : ""}
          ${candidates}
          <div class="service-editor-slot" data-row-id="${escapeHTML(row.row_id)}">${renderServiceEditor(row, context)}</div>
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">\u63A8\u9001\u5BF9\u8C61</div>
          <div class="mode-tabs" role="group" aria-label="\u63A8\u9001\u5BF9\u8C61\u7C7B\u578B">
            <button type="button" class="button button--small ${row.target_mode === "group" ? "is-active" : ""}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="group">\u7FA4\u804A</button>
            <button type="button" class="button button--small ${row.target_mode === "private" ? "is-active" : ""}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="private">\u79C1\u804A</button>
          </div>
          <div class="target-select" data-row-id="${escapeHTML(row.row_id)}" role="listbox" aria-multiselectable="true" aria-disabled="${context.targets.loaded ? "false" : "true"}" tabindex="0">
            <div class="target-options-list">${renderTargetOptions(row, context)}</div>
          </div>
          <div class="chip-list target-chip-list" data-row-id="${escapeHTML(row.row_id)}">${renderSelectedTargets(row, context)}</div>
          ${context.targets.issues.length ? `<div class="target-note">${escapeHTML(context.targets.issues.map((issue) => issue.message).join("\uFF1B"))}</div>` : ""}
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">\u8BA2\u9605\u4EBA</div>
          ${renderSubscriberEditor(row, context)}
        </div>
      </div>

      <div class="sub-card__actions">
        <div class="button-group">
          <label class="switch-row" title="\u542F\u7528">
            <input type="checkbox" class="row-enabled-input" data-row-id="${escapeHTML(row.row_id)}" ${row.enabled ? "checked" : ""} />
            <span>${row.enabled ? "\u5DF2\u542F\u7528" : "\u5DF2\u505C\u7528"}</span>
          </label>
        </div>
        <div class="button-group">
          <button type="button" class="button button--primary button--small" data-action="finish-edit" data-row-id="${escapeHTML(row.row_id)}">\u5B8C\u6210</button>
          <button type="button" class="button button--ghost button--small" data-action="cancel-edit" data-row-id="${escapeHTML(row.row_id)}">\u53D6\u6D88</button>
          <button type="button" class="button button--small" data-action="duplicate-row" data-row-id="${escapeHTML(row.row_id)}">\u590D\u5236</button>
          <button type="button" class="button button--small button--danger" data-action="delete-row" data-row-id="${escapeHTML(row.row_id)}">\u5220\u9664</button>
        </div>
      </div>
    </article>
  `;
  }

  // ../plugins/builtin/subscription_hub/web/render/row-view.js
  function renderRowView(row, context) {
    const map = context.targetMap;
    const title = row.name || row.uid || "\u672A\u6821\u9A8C UP";
    const subtitle = row.uid ? `UID ${row.uid}` : "\u8F93\u5165 UID \u6216 Bilibili \u7528\u6237\u540D\u540E\u6821\u9A8C";
    const upAvatar = avatarHTML(row.avatar_url, title, "avatar--up", title);
    const services = row.service_mode === "mixed" ? '<span class="service-tag">\u76EE\u6807\u914D\u7F6E\u4E0D\u540C</span>' : serviceTagsHTML(row.services);
    const targetSummaryItems = row.targets.map((target) => ({
      avatar_url: targetAvatar(target, map),
      label: targetDisplay(target, map)
    }));
    const targetStack = avatarStackHTML(targetSummaryItems, 5, "avatar--target", (item) => item.avatar_url, (item) => item.label);
    const targetLabel = row.targets.length ? `${row.targets.length} \u4E2A\u63A8\u9001\u5BF9\u8C61` : "\u672A\u9009\u62E9\u63A8\u9001\u5BF9\u8C61";
    const subscriberItems = row.subscriber_ids.map((id) => ({
      id,
      avatar_url: subscriberAvatarURL(context.subscriberAvatars, id)
    }));
    const subscriberStack = row.subscriber_ids.length ? avatarStackHTML(subscriberItems, 5, "avatar--subscriber", (item) => item.avatar_url, (item) => `QQ ${item.id}`) : '<span class="chip chip--success">\u7CFB\u7EDF\u8BA2\u9605</span>';
    const subscriberLabel = row.subscriber_ids.length ? `${row.subscriber_ids.length} \u4F4D\u8BA2\u9605\u4EBA` : "\u7CFB\u7EDF\u8BA2\u9605";
    return `
    <article class="sub-card ${row.enabled ? "" : "sub-card--disabled"}" data-row-id="${escapeHTML(row.row_id)}">
      <div class="sub-card__head">
        ${upAvatar}
        <div class="sub-card__meta">
          <strong>${escapeHTML(title)}</strong>
          <small>${escapeHTML(subtitle)}</small>
        </div>
        <div class="sub-card__status">
          ${row.resolved ? '<span class="badge">\u5DF2\u6821\u9A8C</span>' : ""}
          ${renderValidationBadge(row, context)}
          <label class="switch-row" title="\u542F\u7528">
            <input type="checkbox" class="row-enabled-input" data-row-id="${escapeHTML(row.row_id)}" ${row.enabled ? "checked" : ""} />
          </label>
        </div>
      </div>

      <div class="sub-card__body">
        <div class="sub-card__section">
          <div class="sub-card__section-title">\u63A8\u9001\u7C7B\u578B</div>
          <div class="sub-card__services">${services}</div>
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">\u63A8\u9001\u5BF9\u8C61</div>
          <div class="sub-card__targets-summary">
            ${targetStack}
            <span class="sub-card__summary-label">${escapeHTML(targetLabel)}</span>
          </div>
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">\u8BA2\u9605\u4EBA</div>
          <div class="sub-card__subscribers-summary">
            ${subscriberStack}
            <span class="sub-card__summary-label">${escapeHTML(subscriberLabel)}</span>
          </div>
        </div>
      </div>

      <div class="sub-card__actions">
        <button type="button" class="button button--primary button--small" data-action="edit-row" data-row-id="${escapeHTML(row.row_id)}">\u7F16\u8F91</button>
        <button type="button" class="button button--small" data-action="duplicate-row" data-row-id="${escapeHTML(row.row_id)}">\u590D\u5236</button>
        <button type="button" class="button button--small button--danger" data-action="delete-row" data-row-id="${escapeHTML(row.row_id)}">\u5220\u9664</button>
      </div>
    </article>
  `;
  }

  // ../plugins/builtin/subscription_hub/web/app-entry.js
  var elements = {
    statusText: document.getElementById("status-text"),
    enabledInput: document.getElementById("enabled-input"),
    metricEnabled: document.getElementById("metric-enabled"),
    metricSubscriptions: document.getElementById("metric-subscriptions"),
    metricTargets: document.getElementById("metric-targets"),
    metricValidation: document.getElementById("metric-validation"),
    targetsReloadButton: document.getElementById("targets-reload-button"),
    searchInput: document.getElementById("subscription-search-input"),
    statusFilter: document.getElementById("status-filter-input"),
    serviceFilter: document.getElementById("service-filter-input"),
    addButton: document.getElementById("add-subscription-button"),
    list: document.getElementById("subscription-list"),
    dirtyState: document.getElementById("dirty-state"),
    reloadButton: document.getElementById("reload-button"),
    resetButton: document.getElementById("reset-button"),
    manualCheckButton: document.getElementById("manual-check-button"),
    previewButton: document.getElementById("preview-button"),
    saveButton: document.getElementById("save-button")
  };
  var state = {
    defaultSettings: { enabled: true, subscriptions: [] },
    settings: { enabled: true, subscriptions: [] },
    rows: [],
    targets: {
      loaded: false,
      available: false,
      groups: [],
      private_users: [],
      issues: []
    },
    identities: {
      subscriberAvatars: /* @__PURE__ */ new Map()
    },
    requests: {
      pending: /* @__PURE__ */ new Map(),
      resolveTimers: /* @__PURE__ */ new Map(),
      savingRequestId: ""
    },
    filters: {
      search: "",
      status: "all",
      service: "all"
    },
    ui: {
      loaded: false,
      dirty: false,
      rowCounter: 0
    }
  };
  var bridge;
  function renderContext() {
    const liveTargets = targetMap(state.targets);
    return {
      targets: state.targets,
      targetMap: liveTargets,
      targetsLoaded: state.targets.loaded,
      subscriberAvatars: state.identities.subscriberAvatars
    };
  }
  function nextRowId() {
    state.ui.rowCounter += 1;
    return `row-${Date.now()}-${state.ui.rowCounter}`;
  }
  function nextFrame(callback) {
    const schedule = window.requestAnimationFrame || ((fn) => window.setTimeout(fn, 0));
    schedule.call(window, callback);
  }
  function scrollRowIntoCenter(row) {
    nextFrame(() => {
      const element = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`);
      if (element && typeof element.scrollIntoView === "function") {
        element.scrollIntoView({ block: "center", behavior: "smooth" });
      }
    });
  }
  function findRow(rowId) {
    return state.rows.find((row) => row.row_id === rowId);
  }
  function setStatus(text) {
    elements.statusText.textContent = text;
  }
  function currentValidation() {
    return validateRows(state.rows, renderContext());
  }
  function renderPageState() {
    elements.enabledInput.checked = state.settings.enabled !== false;
    elements.metricEnabled.textContent = state.settings.enabled === false ? "\u505C\u7528" : "\u542F\u7528";
    elements.metricSubscriptions.textContent = `${state.rows.length} / ${(state.settings.subscriptions || []).length}`;
    elements.metricTargets.textContent = state.targets.loaded ? `${state.targets.groups.length} \u7FA4\u804A / ${state.targets.private_users.length} \u79C1\u804A` : "\u672A\u8F7D\u5165";
    const validation = currentValidation();
    elements.metricValidation.textContent = validation.ok ? "\u53EF\u4FDD\u5B58" : "\u9700\u5904\u7406";
    elements.saveButton.disabled = !state.ui.loaded || !validation.ok || state.requests.savingRequestId !== "";
    elements.dirtyState.textContent = state.requests.savingRequestId ? "\u6B63\u5728\u4FDD\u5B58" : state.ui.dirty ? "\u8BBE\u7F6E\u6709\u4FEE\u6539" : state.ui.loaded ? "\u8BBE\u7F6E\u5DF2\u540C\u6B65" : "\u7B49\u5F85\u8F7D\u5165";
  }
  function renderRow(row) {
    const context = renderContext();
    return row.edit_mode ? renderRowEdit(row, context) : renderRowView(row, context);
  }
  function render() {
    renderPageState();
    const visibleRows = state.rows.filter(rowVisible);
    elements.list.innerHTML = visibleRows.length ? visibleRows.map(renderRow).join("") : renderEmptyState();
  }
  function markDirty() {
    state.ui.dirty = true;
    render();
  }
  function markDirtyWithRowRefresh(row, refresh) {
    state.ui.dirty = true;
    refresh(row);
  }
  function rowSearchText(row) {
    const map = targetMap(state.targets);
    return [
      row.uid,
      row.name,
      row.query,
      ...row.targets.map((target) => `${target.target_id} ${target.target_name || ""}`),
      ...row.targets.map((target) => map.get(target.key)?.label || ""),
      ...row.subscriber_ids
    ].join(" ").toLowerCase();
  }
  function rowVisible(row) {
    const query = trim(state.filters.search).toLowerCase();
    if (query && !rowSearchText(row).includes(query)) {
      return false;
    }
    if (state.filters.status === "enabled" && !row.enabled) {
      return false;
    }
    if (state.filters.status === "disabled" && row.enabled) {
      return false;
    }
    if (state.filters.service !== "all") {
      const services = row.service_mode === "mixed" ? row.targets.flatMap((target) => target.services) : row.services;
      if (!services.includes("all") && !services.includes(state.filters.service)) {
        return false;
      }
    }
    return true;
  }
  function refreshValidationAndPage(card, row) {
    const validationSlot = card.querySelector(".row-validation-slot");
    if (validationSlot) {
      validationSlot.innerHTML = renderRowValidation(row, renderContext());
    }
    renderPageState();
  }
  function refreshRowTargetEditor(row) {
    const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`);
    if (!card) {
      render();
      return;
    }
    const context = renderContext();
    const targetList = card.querySelector(".target-select");
    const targetScrollTop = targetList ? targetList.scrollTop : 0;
    const activeKey = document.activeElement && document.activeElement.dataset ? document.activeElement.dataset.targetKey : "";
    const optionsList = card.querySelector(".target-options-list");
    if (optionsList) {
      optionsList.innerHTML = renderTargetOptions(row, context);
    }
    const targetChips = card.querySelector(".target-chip-list");
    if (targetChips) {
      targetChips.innerHTML = renderSelectedTargets(row, context);
    }
    const serviceSlot = card.querySelector(".service-editor-slot");
    if (serviceSlot) {
      serviceSlot.innerHTML = renderServiceEditor(row, context);
    }
    refreshValidationAndPage(card, row);
    if (targetList) {
      targetList.scrollTop = targetScrollTop;
    }
    if (activeKey) {
      const activeOption = card.querySelector(`.target-option[data-target-key="${selectorValue(activeKey)}"]`);
      if (activeOption) {
        activeOption.focus({ preventScroll: true });
      }
    }
  }
  function refreshRowServiceEditor(row) {
    const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`);
    if (!card) {
      render();
      return;
    }
    const active = document.activeElement;
    const activeSelector = active && active.classList && active.classList.contains("service-checkbox") ? `.service-checkbox[data-row-id="${selectorValue(active.dataset.rowId)}"][data-target-key="${selectorValue(active.dataset.targetKey)}"][value="${selectorValue(active.value)}"]` : "";
    const serviceSlot = card.querySelector(".service-editor-slot");
    if (serviceSlot) {
      serviceSlot.innerHTML = renderServiceEditor(row, renderContext());
    }
    refreshValidationAndPage(card, row);
    if (activeSelector) {
      const nextActive = card.querySelector(activeSelector);
      if (nextActive) {
        nextActive.focus({ preventScroll: true });
      }
    }
  }
  function refreshRowSubscriberEditor(row) {
    const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`);
    if (!card) {
      render();
      return;
    }
    const chips = card.querySelector(".subscriber-chip-list");
    if (chips) {
      chips.innerHTML = renderSubscriberChips(row, renderContext());
    }
    refreshValidationAndPage(card, row);
  }
  function beginEdit(row) {
    row._editSnapshot = cloneRow(row);
    row.edit_mode = true;
    render();
    scrollRowIntoCenter(row);
  }
  function cancelEdit(row) {
    if (row._editSnapshot) {
      const snapshot = row._editSnapshot;
      const preservedRowId = row.row_id;
      Object.assign(row, snapshot);
      row.row_id = preservedRowId;
      row._editSnapshot = null;
      row.edit_mode = false;
    }
    render();
  }
  function finishEdit(row) {
    const errors = validateRow(row, renderContext());
    if (errors.length) {
      setStatus(errors[0] || "\u884C\u672A\u901A\u8FC7\u6821\u9A8C");
      render();
      return;
    }
    row._editSnapshot = null;
    row.edit_mode = false;
    markDirty();
  }
  function readCheckedServices(rowId, targetKeyValue, changedService, isChecked) {
    if (changedService === "all") {
      return isChecked ? ["all"] : [];
    }
    const checkedServices = [...elements.list.querySelectorAll(`.service-checkbox[data-row-id="${selectorValue(rowId)}"][data-target-key="${selectorValue(targetKeyValue)}"]:checked`)].map((input) => input.value).filter((service) => service !== "all");
    if (SERVICE_TYPES.every((service) => checkedServices.includes(service))) {
      return ["all"];
    }
    return SERVICE_TYPES.filter((service) => checkedServices.includes(service));
  }
  function updateService(row, targetKeyValue, changedService, isChecked) {
    if (targetKeyValue === "common") {
      row.service_mode = "common";
      row.services = readCheckedServices(row.row_id, "common", changedService, isChecked);
      for (const target2 of row.targets) {
        target2.services = row.services;
      }
      return;
    }
    const target = row.targets.find((item) => item.key === targetKeyValue);
    if (target) {
      target.services = readCheckedServices(row.row_id, targetKeyValue, changedService, isChecked);
      const serviceKeys = unique(row.targets.map((item) => servicesKey(item.services)));
      row.service_mode = serviceKeys.length > 1 ? "mixed" : "common";
      if (row.service_mode === "common" && row.targets[0]) {
        row.services = row.targets[0].services;
      }
    }
  }
  function requestTargets() {
    setStatus("\u6B63\u5728\u5237\u65B0\u63A8\u9001\u5BF9\u8C61\u2026");
    bridge.reloadTargets();
  }
  function requestBilibiliResolve(row, immediate) {
    const query = trim(row.query);
    if (!query) {
      row.resolved = false;
      row.resolve_state = "error";
      row.resolve_message = "\u8BF7\u586B\u5199 UID \u6216 Bilibili \u7528\u6237\u540D\u3002";
      render();
      return;
    }
    const run = () => {
      const requestId = bridge.nextRequestId("bilibili-user");
      state.requests.pending.set(requestId, { kind: "bilibili-user", row_id: row.row_id, query });
      row.resolve_state = "checking";
      row.resolve_message = "\u6B63\u5728\u6821\u9A8C UP\u2026";
      row.candidates = [];
      row.resolved = false;
      render();
      bridge.resolveBilibiliUser(query, requestId);
    };
    if (immediate) {
      run();
      return;
    }
    clearTimeout(state.requests.resolveTimers.get(row.row_id));
    state.requests.resolveTimers.set(row.row_id, setTimeout(run, 450));
  }
  function applyResolvedUser(row, user) {
    row.uid = trim(user.uid);
    row.name = trim(user.name);
    row.avatar_url = trim(user.avatar_url);
    row.query = row.name || row.uid;
    row.resolved = Boolean(row.uid && row.name);
    row.resolve_state = row.resolved ? "resolved" : "error";
    row.candidates = [];
  }
  function applyBilibiliResolved(message) {
    const request = state.requests.pending.get(message.request_id);
    if (!request || request.kind !== "bilibili-user") {
      return;
    }
    state.requests.pending.delete(message.request_id);
    const row = findRow(request.row_id);
    if (!row || request.query !== message.payload.query) {
      return;
    }
    if (message.payload.exact && message.payload.user) {
      applyResolvedUser(row, message.payload.user);
      row.resolve_message = "UP \u5DF2\u6821\u9A8C\u3002";
    } else {
      row.resolved = false;
      row.resolve_state = "error";
      row.resolve_message = message.payload.message || "\u8BF7\u9009\u62E9\u4E00\u4E2A\u5019\u9009 UP \u540E\u4FDD\u5B58\u3002";
      row.candidates = Array.isArray(message.payload.candidates) ? message.payload.candidates : [];
    }
    markDirty();
  }
  function addTargetToRow(row, liveTarget) {
    if (row.targets.some((target) => target.key === liveTarget.key)) {
      return;
    }
    const services = row.service_mode === "mixed" ? ["all"] : row.services;
    row.targets.push({
      key: liveTarget.key,
      subscription_id: "",
      target_type: liveTarget.target_type,
      target_id: liveTarget.target_id,
      target_name: liveTarget.label,
      services: normalizeServices(services)
    });
  }
  function updateTargetsFromSelect(row, selectedKeys) {
    const liveTargets = currentTargetsForMode(state.targets, row.target_mode);
    const liveMap = new Map(liveTargets.map((target) => [target.key, target]));
    row.targets = row.targets.filter((target) => target.target_type !== row.target_mode || selectedKeys.includes(target.key));
    for (const key of selectedKeys) {
      const liveTarget = liveMap.get(key);
      if (liveTarget) {
        addTargetToRow(row, liveTarget);
      }
    }
  }
  function toggleTarget(row, targetKeyValue) {
    const currentModeKeys = row.targets.filter((target) => target.target_type === row.target_mode).map((target) => target.key);
    const selected = new Set(currentModeKeys);
    if (selected.has(targetKeyValue)) {
      selected.delete(targetKeyValue);
    } else {
      selected.add(targetKeyValue);
    }
    updateTargetsFromSelect(row, [...selected]);
  }
  function addSubscriber(row, input) {
    if (!input) {
      return;
    }
    const id = trim(input.value);
    if (!numericPattern.test(id)) {
      setStatus("\u8BA2\u9605\u4EBA QQ \u53F7\u4E0D\u6B63\u786E");
      return;
    }
    row.subscriber_ids = unique([...row.subscriber_ids, id]);
    input.value = "";
    markDirtyWithRowRefresh(row, refreshRowSubscriberEditor);
  }
  function saveSettings() {
    const validation = currentValidation();
    if (!validation.ok) {
      setStatus(validation.errors[0] || "\u8BBE\u7F6E\u672A\u901A\u8FC7\u6821\u9A8C");
      render();
      return;
    }
    const identityRequests = buildIdentityRequests(state.rows);
    if (!identityRequests.length) {
      const payload = buildSettingsPayload(state.settings, state.rows, targetMap(state.targets));
      state.requests.savingRequestId = bridge.nextRequestId("settings-save");
      bridge.saveSettings(payload, state.requests.savingRequestId);
      render();
      return;
    }
    const requestId = bridge.nextRequestId("protocol-identities");
    state.requests.pending.set(requestId, { kind: "save-identities", expected: identityRequests });
    state.requests.savingRequestId = requestId;
    setStatus("\u6B63\u5728\u5237\u65B0\u8BA2\u9605\u4EBA\u8EAB\u4EFD\u2026");
    bridge.resolveIdentities(identityRequests, requestId);
    render();
  }
  function applyIdentitiesResolved(message) {
    const request = state.requests.pending.get(message.request_id);
    if (!request || request.kind !== "save-identities") {
      return;
    }
    state.requests.pending.delete(message.request_id);
    state.requests.savingRequestId = "";
    const issues = Array.isArray(message.payload.issues) ? message.payload.issues : [];
    const items = Array.isArray(message.payload.items) ? message.payload.items : [];
    for (const item of items) {
      if (item.user_id && item.avatar_url) {
        state.identities.subscriberAvatars.set(trim(item.user_id), trim(item.avatar_url));
      }
    }
    const received = new Set(items.map((item) => identityKey(item.target_type, item.target_id, item.user_id)));
    const missing = request.expected.filter((item) => !received.has(identityKey(item.target_type, item.target_id, item.user_id)));
    if (issues.length || missing.length) {
      setStatus(issues[0] && issues[0].message ? issues[0].message : "\u8BA2\u9605\u4EBA\u8EAB\u4EFD\u5237\u65B0\u5931\u8D25");
      render();
      return;
    }
    const payload = buildSettingsPayload(state.settings, state.rows, targetMap(state.targets));
    state.requests.savingRequestId = bridge.nextRequestId("settings-save");
    bridge.saveSettings(payload, state.requests.savingRequestId);
    render();
  }
  function loadSettingsIntoRows(settings) {
    state.settings = settings;
    state.rows = buildRowsFromSettings(settings);
    state.identities.subscriberAvatars = collectSubscriberAvatars(settings);
  }
  function applySettingsChanged(message) {
    loadSettingsIntoRows(normalizeSettings(message.payload && message.payload.values));
    state.ui.loaded = true;
    state.ui.dirty = false;
    state.requests.savingRequestId = "";
    setStatus("\u8BBE\u7F6E\u5DF2\u540C\u6B65");
    render();
  }
  function applyHostInit(payload) {
    state.defaultSettings = normalizeSettings(payload.default_config);
    loadSettingsIntoRows(normalizeSettings(payload.settings));
    state.ui.loaded = true;
    state.ui.dirty = false;
    setStatus("\u8BBE\u7F6E\u5DF2\u8F7D\u5165");
    render();
    requestTargets();
  }
  function applyTargetsChanged(payload) {
    state.targets = normalizeTargets(payload);
    setStatus(state.targets.available ? "\u63A8\u9001\u5BF9\u8C61\u5DF2\u5237\u65B0" : "\u63A8\u9001\u5BF9\u8C61\u4E0D\u53EF\u7528");
    render();
  }
  function handleBridgeMessage(message) {
    switch (message.type) {
      case "host.init":
        applyHostInit(message.payload || {});
        return;
      case "settings.changed":
        applySettingsChanged(message);
        return;
      case "protocol.targets.changed":
        applyTargetsChanged(message.payload || {});
        return;
      case "protocol.identities.resolved":
        applyIdentitiesResolved(message);
        return;
      case "bilibili.user.resolved":
        applyBilibiliResolved(message);
        return;
      default:
        return;
    }
  }
  function handleBridgeError(message) {
    state.requests.savingRequestId = "";
    setStatus(message.error && message.error.message || "\u64CD\u4F5C\u5931\u8D25");
    render();
  }
  function handleListClick(event) {
    const button = event.target.closest("button[data-action]");
    if (!button) {
      return;
    }
    const row = findRow(button.dataset.rowId);
    if (!row) {
      return;
    }
    const action = button.dataset.action;
    if (action === "resolve-up") {
      requestBilibiliResolve(row, true);
      return;
    }
    if (action === "choose-candidate") {
      try {
        applyResolvedUser(row, JSON.parse(button.dataset.user || "{}"));
        row.resolve_message = "UP \u5DF2\u6821\u9A8C\u3002";
        markDirty();
      } catch {
        setStatus("\u5019\u9009 UP \u6570\u636E\u4E0D\u6B63\u786E");
      }
      return;
    }
    if (action === "target-mode") {
      row.target_mode = button.dataset.mode === "private" ? "private" : "group";
      render();
      return;
    }
    if (action === "toggle-target") {
      toggleTarget(row, button.dataset.targetKey);
      markDirtyWithRowRefresh(row, refreshRowTargetEditor);
      return;
    }
    if (action === "remove-target") {
      row.targets = row.targets.filter((target) => target.key !== button.dataset.targetKey);
      markDirty();
      return;
    }
    if (action === "add-subscriber") {
      const input = elements.list.querySelector(`.subscriber-input[data-row-id="${selectorValue(row.row_id)}"]`);
      addSubscriber(row, input);
      return;
    }
    if (action === "remove-subscriber") {
      row.subscriber_ids = row.subscriber_ids.filter((id) => id !== button.dataset.userId);
      markDirtyWithRowRefresh(row, refreshRowSubscriberEditor);
      return;
    }
    if (action === "duplicate-row") {
      const copy = cloneRow(row);
      copy.row_id = nextRowId();
      copy.targets = copy.targets.map((target) => ({ ...target, subscription_id: "" }));
      copy.edit_mode = true;
      copy._editSnapshot = null;
      state.rows.push(copy);
      markDirty();
      return;
    }
    if (action === "delete-row") {
      state.rows = state.rows.filter((item) => item.row_id !== row.row_id);
      markDirty();
      return;
    }
    if (action === "edit-row") {
      beginEdit(row);
      return;
    }
    if (action === "finish-edit") {
      finishEdit(row);
      return;
    }
    if (action === "cancel-edit") {
      cancelEdit(row);
    }
  }
  function handleListInput(event) {
    const input = event.target;
    if (input.classList.contains("up-query-input")) {
      const row = findRow(input.dataset.rowId);
      if (!row) {
        return;
      }
      row.query = input.value;
      row.resolved = false;
      row.resolve_state = "idle";
      row.resolve_message = "";
      row.candidates = [];
      state.ui.dirty = true;
      requestBilibiliResolve(row, false);
    }
  }
  function handleListChange(event) {
    const input = event.target;
    const row = findRow(input.dataset.rowId);
    if (!row) {
      return;
    }
    if (input.classList.contains("service-checkbox")) {
      updateService(row, input.dataset.targetKey, input.value, input.checked);
      markDirtyWithRowRefresh(row, refreshRowServiceEditor);
      return;
    }
    if (input.classList.contains("row-enabled-input")) {
      row.enabled = input.checked;
      markDirty();
    }
  }
  function resetToDefault() {
    loadSettingsIntoRows(normalizeSettings(state.defaultSettings));
    state.ui.dirty = true;
    setStatus("\u5DF2\u6062\u590D\u9ED8\u8BA4\u8BBE\u7F6E\uFF0C\u4FDD\u5B58\u540E\u751F\u6548");
    render();
  }
  function bindEvents() {
    elements.enabledInput.addEventListener("change", () => {
      state.settings.enabled = elements.enabledInput.checked;
      markDirty();
    });
    elements.targetsReloadButton.addEventListener("click", requestTargets);
    elements.searchInput.addEventListener("input", () => {
      state.filters.search = elements.searchInput.value;
      render();
    });
    elements.statusFilter.addEventListener("change", () => {
      state.filters.status = elements.statusFilter.value;
      render();
    });
    elements.serviceFilter.addEventListener("change", () => {
      state.filters.service = elements.serviceFilter.value;
      render();
    });
    elements.addButton.addEventListener("click", () => {
      const newRow = createBlankRow(nextRowId());
      state.rows.unshift(newRow);
      markDirty();
      scrollRowIntoCenter(newRow);
    });
    elements.list.addEventListener("click", handleListClick);
    elements.list.addEventListener("input", handleListInput);
    elements.list.addEventListener("change", handleListChange);
    elements.reloadButton.addEventListener("click", () => {
      setStatus("\u6B63\u5728\u91CD\u65B0\u8F7D\u5165\u8BBE\u7F6E\u2026");
      bridge.reloadSettings();
    });
    elements.resetButton.addEventListener("click", resetToDefault);
    elements.manualCheckButton.addEventListener("click", () => {
      setStatus("Bilibili \u4E8B\u4EF6\u6E90\u72B6\u6001\u5728 Web \u4E09\u65B9\u76D1\u63A7\u9875\u9762\u67E5\u770B");
    });
    elements.previewButton.addEventListener("click", () => {
      bridge.openRenderTemplate("plugin.raylea.subscription-hub.bilibili-update");
    });
    elements.saveButton.addEventListener("click", saveSettings);
    elements.list.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" || !event.target.classList.contains("subscriber-input")) {
        return;
      }
      event.preventDefault();
      const row = findRow(event.target.dataset.rowId);
      if (row) {
        addSubscriber(row, event.target);
      }
    });
  }
  bridge = createBridgeClient(window, {
    onMessage: handleBridgeMessage,
    onError: handleBridgeError
  });
  bindEvents();
  render();
  bridge.pageReady();
  window.__subscriptionHubSettingsPage = {
    state,
    normalizeSettings,
    buildRowsFromSettings,
    buildSettingsPayload: (identityItems = []) => {
      void identityItems;
      return buildSettingsPayload(state.settings, state.rows, targetMap(state.targets));
    },
    validateRows: () => validateRows(state.rows, renderContext())
  };
})();
