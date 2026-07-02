(function () {
  var githubBase = "https://github.com/silaswei-io/skills-seed/blob/main/";

  var copy = {
    zh: {
      navStart: "快速开始",
      navOutput: "产物结构",
      navCommands: "命令",
      eyebrow: "Local skills for Claude Code and Codex",
      heroTitle: "让 AI Agent 先理解项目规则，再开始改代码。",
      heroLead: "Skills Seed 从已有代码、Git 历史和检查命中里提取团队真实实践，生成本地可加载的 Skills，让 Agent 按你的项目边界、业务入口和测试习惯工作。",
      primaryCta: "两步开始",
      secondaryCta: "查看命令参考",
      terminalComment: "在已有 Git 项目根目录执行",
      skillEntry: "Agent 入口",
      startKicker: "Workflow",
      startTitle: "两个命令跑完整个流程",
      startLead: "<code>init</code> 和 <code>sync</code> 都会根据当前仓库状态动态切换：首次运行进入配置流程，再次运行会提供查看、重置、续跑或重新同步。",
      stepInitTitle: "初始化",
      stepInitText: "首次运行选择单项目或 workspace、Agent、语言和执行计划；再次运行可查看当前配置或重新初始化。",
      stepSyncTitle: "同步",
      stepSyncText: "首次同步建立第一版上下文；已有产物后执行增量学习；存在未完成状态时可续跑或重来。",
      stepUseTitle: "使用",
      stepUseText: "打开 Claude Code 或 Codex，Agent 会按生成的入口文件读取最小必要项目上下文。",
      valueKicker: "What it captures",
      valueTitle: "沉淀真实项目经验",
      valueLead: "它面向已有项目，不要求你先手写完整规范。仓库事实、团队补充和检查命中会一起形成可持续刷新的 Agent 上下文。",
      featureLearnTitle: "项目规则学习",
      featureLearnText: "从当前代码和 Git 历史中提取 patterns、业务方法、工具方法、测试策略和项目画像。",
      featureOutputTitle: "Agent 可加载产物",
      featureOutputText: "生成 <code>SKILL.md</code> 和 references，让 Claude Code / Codex 按任务读取最小必要上下文。",
      featureWorkspaceTitle: "Workspace 路由",
      featureWorkspaceText: "根仓负责路由和跨项目关系，子项目独立学习，避免上下文互相污染。",
      featureCheckTitle: "变更检查",
      featureCheckText: "用已学习规则检查当前改动，记录 pattern hits，让真正有用的规则更容易进入最终 Skills。",
      outputKicker: "Generated output",
      outputTitle: "生成的 Skills 结构",
      outputLead: "<code>SKILL.md</code> 是 Agent 入口，references 保存更完整的项目画像、规范、业务入口和模式细节。Agent 先读入口，再按任务深入相关参考文件。",
      treeTitle: "Codex 默认输出",
      commandsKicker: "CLI",
      commandsTitle: "主命令索引",
      commandsLead: "首页保留常用入口；完整子命令和参数由命令参考维护。",
      cmdInit: "交互式初始化当前项目或 workspace。",
      cmdSync: "交互式学习当前代码并刷新 Skills。",
      cmdWorkspace: "管理 workspace 子项目。",
      cmdPatterns: "查看、补充、修订或整理模式规则。",
      cmdWorkflow: "添加或更新团队常用任务流程。",
      cmdCheck: "用已学习规则检查当前改动。",
      workspaceTitle: "多项目仓库也能保持上下文边界",
      workspaceLead: "一个根目录下有多个独立 Git 子项目时，选择 workspace 模式。根仓负责编排、路由和跨项目关系，子项目独立保存自己的学习结果。",
      footerPrefix: "更多细节见",
      footerCommands: "命令参考",
      footerAnd: "和",
      footerConfig: "配置参考"
    },
    en: {
      navStart: "Start",
      navOutput: "Output",
      navCommands: "Commands",
      eyebrow: "Local skills for Claude Code and Codex",
      heroTitle: "Make AI agents understand your project rules before they edit code.",
      heroLead: "Skills Seed learns real team practices from code, Git history, and check hits, then generates local Skills that guide agents through your project boundaries, business entries, and validation habits.",
      primaryCta: "Start in two steps",
      secondaryCta: "Command reference",
      terminalComment: "Run from an existing Git project root",
      skillEntry: "Agent entry",
      startKicker: "Workflow",
      startTitle: "Two commands cover the full loop",
      startLead: "<code>init</code> and <code>sync</code> adapt to repository state: first runs open setup, later runs offer inspect, reset, resume, or resync paths.",
      stepInitTitle: "Initialize",
      stepInitText: "Choose project or workspace mode, Agent, language, and execution plan on the first run. Later runs can inspect or reinitialize.",
      stepSyncTitle: "Sync",
      stepSyncText: "First sync builds the initial context. Existing output runs incremental learning. Unfinished state can resume or restart.",
      stepUseTitle: "Use",
      stepUseText: "Open Claude Code or Codex. The agent reads the generated entry file and follows the minimum relevant project context.",
      valueKicker: "What it captures",
      valueTitle: "Turn real project practice into agent context",
      valueLead: "Skills Seed is built for existing repositories. Repository facts, team additions, and check hits become refreshable context for AI agents.",
      featureLearnTitle: "Project learning",
      featureLearnText: "Extract patterns, business methods, utilities, validation strategy, and project profile from current code and Git history.",
      featureOutputTitle: "Agent-loadable output",
      featureOutputText: "Generate <code>SKILL.md</code> and references so Claude Code or Codex can read only the context needed for the current task.",
      featureWorkspaceTitle: "Workspace routing",
      featureWorkspaceText: "The root repository handles routing and cross-project relationships while child projects learn independently.",
      featureCheckTitle: "Change checks",
      featureCheckText: "Check current diffs with learned rules and record pattern hits so useful rules become more visible.",
      outputKicker: "Generated output",
      outputTitle: "Generated Skills structure",
      outputLead: "<code>SKILL.md</code> is the agent entry point. references keeps the fuller project profile, specs, business entries, and pattern details. Agents read the entry first, then open focused references when needed.",
      treeTitle: "Default Codex output",
      commandsKicker: "CLI",
      commandsTitle: "Main commands",
      commandsLead: "This page keeps the common entry points visible. The full subcommand and flag details live in the command reference.",
      cmdInit: "Interactively initialize the current project or workspace.",
      cmdSync: "Interactively learn current code and refresh Skills.",
      cmdWorkspace: "Manage workspace child projects.",
      cmdPatterns: "Inspect, add, revise, or curate pattern rules.",
      cmdWorkflow: "Add or update common team workflows.",
      cmdCheck: "Check current changes with learned rules.",
      workspaceTitle: "Keep context boundaries in multi-project workspaces",
      workspaceLead: "When one root contains multiple independent Git projects, choose workspace mode. The root handles orchestration, routing, and cross-project relationships; each child keeps its own learning output.",
      footerPrefix: "More details:",
      footerCommands: "command reference",
      footerAnd: "and",
      footerConfig: "configuration reference"
    }
  };

  function preferredLanguage() {
    var params = new URLSearchParams(window.location.search);
    var queryLang = params.get("lang");
    if (queryLang === "en" || queryLang === "zh") {
      return queryLang;
    }
    if (/README\.en\.md/i.test(document.referrer)) {
      return "en";
    }
    if (/README\.md/i.test(document.referrer)) {
      return "zh";
    }
    return localStorage.getItem("skills-seed-lang") || "zh";
  }

  function setLanguage(lang) {
    var selected = lang === "en" ? "en" : "zh";
    var dict = copy[selected];
    document.documentElement.lang = selected === "en" ? "en" : "zh-CN";
    document.querySelectorAll("[data-i18n]").forEach(function (node) {
      var key = node.getAttribute("data-i18n");
      if (dict[key]) {
        node.textContent = dict[key];
      }
    });
    document.querySelectorAll("[data-i18n-html]").forEach(function (node) {
      var key = node.getAttribute("data-i18n-html");
      if (dict[key]) {
        node.innerHTML = dict[key];
      }
    });
    document.querySelectorAll("[data-lang-toggle]").forEach(function (button) {
      button.textContent = selected === "en" ? "中" : "EN";
      button.setAttribute("aria-label", selected === "en" ? "切换到中文" : "Switch to English");
    });
    document.querySelectorAll('[data-doc-link="readme"]').forEach(function (link) {
      link.href = githubBase + (selected === "en" ? "README.en.md" : "README.md");
    });
    document.querySelectorAll('[data-doc-link="commands"]').forEach(function (link) {
      link.href = selected === "en" ? "../COMMANDS.EN.md" : "../COMMANDS.md";
    });
    document.querySelectorAll('[data-doc-link="config"]').forEach(function (link) {
      link.href = selected === "en" ? "../CONFIGURATION.EN.md" : "../CONFIGURATION.md";
    });
    localStorage.setItem("skills-seed-lang", selected);
  }

  function preferredTheme() {
    var stored = localStorage.getItem("skills-seed-theme");
    if (stored === "dark" || stored === "light") {
      return stored;
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function setTheme(theme) {
    var selected = theme === "dark" ? "dark" : "light";
    document.documentElement.setAttribute("data-theme", selected);
    document.querySelectorAll("[data-theme-icon]").forEach(function (node) {
      node.textContent = selected === "dark" ? "☀" : "◐";
    });
    localStorage.setItem("skills-seed-theme", selected);
  }

  setTheme(preferredTheme());
  setLanguage(preferredLanguage());

  document.querySelectorAll("[data-theme-toggle]").forEach(function (button) {
    button.addEventListener("click", function () {
      var current = document.documentElement.getAttribute("data-theme");
      setTheme(current === "dark" ? "light" : "dark");
    });
  });

  document.querySelectorAll("[data-lang-toggle]").forEach(function (button) {
    button.addEventListener("click", function () {
      setLanguage(document.documentElement.lang === "en" ? "zh" : "en");
    });
  });
})();
