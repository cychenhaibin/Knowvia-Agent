import React from 'react';
import appLogo from './assets/logo.png';

const capabilities = [
  {
    title: 'Yuque RAG',
    description: '连接语雀知识库，同步文档与片段，在问题真正进入执行前先建立可信知识上下文。',
    points: ['Space 连接与同步', '片段级检索与命中摘要', '内部知识与任务目标联动'],
    accent: 'yuque',
  },
  {
    title: 'Skills Layer',
    description: '把总结、行动计划、自定义提示与 GitHub 导入技能挂到同一层，按任务切换执行策略。',
    points: ['总结 / 行动计划', '自定义 Prompt 封装', 'GitHub 导入与复用'],
    accent: 'skills',
  },
  {
    title: 'Visible Runs',
    description: 'Run 是核心对象。规划、检索、证据整理、报告写作逐步展开，而不是黑箱式 Agent 回答。',
    points: ['任务建模为 Run', '步骤状态持续更新', '结果与过程一并保存'],
    accent: 'runs',
  },
  {
    title: 'Streaming UX',
    description: '回复文本、步骤进度与检索信号实时反馈，让长任务执行过程始终可感知。',
    points: ['SSE 事件流', '状态增量刷新', '长任务过程不中断'],
    accent: 'stream',
  },
];

const workflow = [
  '连接知识库',
  '发起任务',
  '自动规划',
  '检索内部知识',
  '补充外部证据',
  '整理证据',
  '生成报告',
  '交付结果',
];

const previews = [
  {
    eyebrow: 'Workspace',
    title: '主工作台',
    description: '在同一界面里输入任务、挂载知识、选择 Skill，并观察流式执行状态持续推进。',
    tags: ['Task Input', 'Skills', 'Knowledge Mount', 'Model'],
  },
  {
    eyebrow: 'Run Detail',
    title: 'Run 详情页',
    description: '按时间线查看 planning、search、merge、writer 等步骤，并随时回看 final report 与 sources。',
    tags: ['Timeline', 'Sources', 'Final Report'],
  },
  {
    eyebrow: 'Knowledge',
    title: '知识库页',
    description: '管理 Yuque 连接、同步状态与文档规模，为后续 hybrid research 持续准备内部证据。',
    tags: ['Yuque Sync', 'KB Status', 'Document Stats'],
  },
];

const scenarios = [
  {
    title: '内部研究',
    description: '输入研究问题，系统检索内部知识并组织证据，输出可交付结论。',
    tags: ['Yuque', 'Evidence', 'Report'],
  },
  {
    title: '竞品分析',
    description: '把语雀沉淀与近期外部动态合并，形成结构化对比与洞察摘要。',
    tags: ['Web', 'Sources', 'Merge'],
  },
  {
    title: '增长复盘',
    description: '汇总内部复盘文档与最近信号，生成可追溯的判断与下一步建议。',
    tags: ['RAG', 'Timeline', 'Delivery'],
  },
  {
    title: '行动计划生成',
    description: '围绕目标自动规划、补证据、写行动建议，让任务从问题走到执行方案。',
    tags: ['Action Plan', 'Skill', 'Run'],
  },
];

const compareCards = [
  {
    title: '普通聊天机器人',
    lines: ['擅长即时回答', '过程通常不可见', '难回到结构化来源'],
  },
  {
    title: '通用 Agent 框架',
    lines: ['更偏开发者配置', '灵活但上手复杂', '不天然面向知识交付'],
  },
  {
    title: 'Knowvia',
    lines: ['面向知识工作台', '显式执行步骤', '结果与来源同步交付'],
    featured: true,
  },
];

const integrations = [
  'Yuque',
  'Google',
  'Microsoft',
  'GitHub',
  'Go API',
  'Python RAG',
  'Postgres + pgvector',
  'Redis',
];

const stats = [
  { value: 'kb_only / web_only / hybrid', label: '任务在规划阶段被明确分类' },
  { value: 'SSE 实时事件流', label: '步骤状态与报告增量同屏更新' },
  { value: 'Sources 可追溯', label: '每条结论都能回到知识片段或外部证据' },
];

function Icon({ type }) {
  const common = {
    width: 24,
    height: 24,
    viewBox: '0 0 24 24',
    fill: 'none',
    xmlns: 'http://www.w3.org/2000/svg',
  };

  switch (type) {
    case 'yuque':
      return (
        <svg {...common}>
          <rect x="3.5" y="4" width="17" height="16" rx="4" stroke="currentColor" strokeWidth="1.5" />
          <path d="M8 8.5H16M8 12H13.5M8 15.5H12" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      );
    case 'skills':
      return (
        <svg {...common}>
          <path d="M12 3.5L14.6 8.65L20.25 9.47L16.12 13.5L17.1 19.2L12 16.52L6.9 19.2L7.88 13.5L3.75 9.47L9.4 8.65L12 3.5Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
        </svg>
      );
    case 'runs':
      return (
        <svg {...common}>
          <path d="M5 6.5H11M5 12H11M5 17.5H11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <circle cx="16.5" cy="6.5" r="1.5" fill="currentColor" />
          <circle cx="16.5" cy="12" r="1.5" fill="currentColor" />
          <circle cx="16.5" cy="17.5" r="1.5" fill="currentColor" />
        </svg>
      );
    default:
      return (
        <svg {...common}>
          <path d="M4 12H9L11.5 6L14.5 18L17 12H20" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
  }
}

function App() {
  return (
    <div className="page-shell">
      <div className="page-noise" aria-hidden="true" />

      <header className="topbar">
        <a className="brand" href="#top" aria-label="Knowvia 首页">
          <span className="brand-mark">
            <img src={appLogo} alt="" className="brand-mark-image" />
          </span>
          <span>Knowvia</span>
        </a>

        <nav className="nav-links" aria-label="主导航">
          <a href="#capabilities">能力</a>
          <a href="#workflow">工作流</a>
          <a href="#scenarios">场景</a>
          <a href="#integrations">集成</a>
          <a href="#cta">开始体验</a>
        </nav>

        <div className="nav-actions">
          <a className="button button-primary" href="#cta">
            立即体验
          </a>
        </div>
      </header>

      <main id="top">
        <section className="hero section">
          <div className="hero-copy reveal">
            <div className="eyebrow">Knowledge-driven execution workspace</div>
            <h1>把知识变成真正可执行的工作流</h1>
            <p className="hero-description">
              Knowvia 将语雀知识库、RAG 检索、Skills 与显式执行流程整合在同一工作台中，帮助用户从提问、检索到结构化交付完成整条链路，并确保每一步有迹可循。
            </p>

            <div className="hero-actions">
              <a className="button button-primary" href="#cta">
                立即体验
              </a>
              <a className="button button-secondary" href="#preview">
                查看界面预览
              </a>
            </div>

            <div className="hero-note">
              不是泛用聊天机器人，也不是开发者框架首页。它围绕知识、执行、证据和交付展开。
            </div>

            <div className="stats-row hero-stats" aria-label="核心指标">
              {stats.map((item) => (
                <article key={item.value} className="stat-card">
                  <strong>{item.value}</strong>
                  <span>{item.label}</span>
                </article>
              ))}
            </div>
          </div>

          <div className="hero-visual reveal reveal-delay">
            <div className="workspace-window">
              <div className="window-toolbar">
                <div className="window-dots">
                  <span />
                  <span />
                  <span />
                </div>
                <div className="window-title">Run Workspace</div>
                <div className="window-status">Streaming</div>
              </div>

              <div className="workspace-body">
                <div className="workspace-main">
                  <div className="panel task-panel">
                    <div className="panel-label">Task</div>
                    <div className="task-query">
                      把语雀里的 Q2 增长复盘和最近 7 天竞品动态整合成行动计划
                    </div>
                    <div className="task-meta">
                      <span>Mounted: Yuque KB</span>
                      <span>Skill: Action Plan</span>
                      <span>Model: QuickQue Lite</span>
                    </div>
                  </div>

                  <div className="panel step-panel">
                    <div className="panel-head">
                      <span className="panel-label">Run Steps</span>
                      <span className="panel-state">live</span>
                    </div>
                    <div className="step-list">
                      {['Planning', 'Yuque Search', 'Web Evidence', 'Evidence Merge', 'Report Writer'].map(
                        (step, index) => (
                          <div
                            key={step}
                            className={`step-item ${index < 4 ? 'is-complete' : 'is-active'}`}
                          >
                            <span className="step-bullet" />
                            <div>
                              <strong>{step}</strong>
                              <p>
                                {index === 0 && 'Classified goal as hybrid_research.'}
                                {index === 1 && 'Matched 6 snippets from 2 Yuque repos.'}
                                {index === 2 && 'Fetched recent competitor signals and pages.'}
                                {index === 3 && 'Merged internal notes with external evidence.'}
                                {index === 4 && 'Drafting structured action plan and delivery.'}
                              </p>
                            </div>
                          </div>
                        ),
                      )}
                    </div>
                  </div>
                </div>

                <div className="workspace-side">
                  <div className="panel report-panel">
                    <div className="panel-label">Final Report</div>
                    <div className="report-card">
                      <div className="report-title">Q2 增长行动建议</div>
                      <ul>
                        <li>优先修复漏斗中部流失，并保留现有高转化素材结构。</li>
                        <li>竞品近 7 天侧重模板化 onboarding，建议补齐引导实验。</li>
                        <li>下周优先推进落地页 AB、内容沉淀与周报追踪机制。</li>
                      </ul>
                    </div>
                  </div>

                  <div className="panel sources-panel">
                    <div className="panel-head">
                      <span className="panel-label">Sources</span>
                      <span className="source-count">12 linked</span>
                    </div>
                    <div className="source-card">
                      <span className="source-tag source-tag-yuque">Yuque snippet</span>
                      <strong>Q2 增长复盘 / Onboarding 漏斗</strong>
                      <p>“新用户在第二步转化下滑明显，建议缩短引导路径并强化模板入口。”</p>
                    </div>
                    <div className="source-card">
                      <span className="source-tag source-tag-web">Web source</span>
                      <strong>Competitor launch notes / last 7 days</strong>
                      <p>竞品在首页新增 role-based template 入口，并强化导入流程。</p>
                    </div>
                    <div className="source-card">
                      <span className="source-tag source-tag-report">Structured report</span>
                      <strong>Action Plan v1</strong>
                      <p>按优先级整理为本周实验、内容整理、数据跟踪三条执行线。</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div className="workspace-phone">
              <div className="phone-shell">
                <div className="phone-notch" />
                <div className="phone-header">
                  <div>
                    <div className="phone-kicker">Knowvia</div>
                    <strong>Run Preview</strong>
                  </div>
                  <span className="phone-live">live</span>
                </div>

                <div className="phone-task-card">
                  <span className="phone-pill">Yuque KB + Action Plan</span>
                  <p>把 Q2 增长复盘和最近 7 天竞品动态整理成行动计划</p>
                </div>

                <div className="phone-steps">
                  <div className="phone-step">
                    <span className="phone-step-dot" />
                    <div>
                      <strong>Planning</strong>
                      <p>Classified as hybrid_research</p>
                    </div>
                  </div>
                  <div className="phone-step">
                    <span className="phone-step-dot" />
                    <div>
                      <strong>Yuque + Web</strong>
                      <p>Matched snippets and fetched external evidence</p>
                    </div>
                  </div>
                  <div className="phone-step">
                    <span className="phone-step-dot phone-step-dot-active" />
                    <div>
                      <strong>Report Writer</strong>
                      <p>Writing final action plan with linked sources</p>
                    </div>
                  </div>
                </div>

                <div className="phone-source-card">
                  <span className="phone-pill phone-pill-soft">Sources linked</span>
                  <strong>Yuque snippet + Web source + Structured report</strong>
                  <p>每条结论都可回到知识片段或外部证据。</p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="section" id="capabilities">
          <div className="section-heading reveal">
            <div className="eyebrow">Core capabilities</div>
            <h2>不是泛化 AI 壳子，而是围绕知识与执行闭环设计</h2>
            <p>
              Knowvia 的核心不是“回答得像 AI”，而是把检索、步骤、证据和交付组织成一个可持续复用的工作台。
            </p>
          </div>

          <div className="capability-grid">
            {capabilities.map((item, index) => (
              <article key={item.title} className={`capability-card reveal reveal-stagger-${index + 1}`}>
                <div className="capability-icon">
                  <Icon type={item.accent} />
                </div>
                <div className="capability-title-row">
                  <h3>{item.title}</h3>
                  <span className="capability-index">0{index + 1}</span>
                </div>
                <p>{item.description}</p>
                <div className="signal-lines" aria-hidden="true">
                  <span />
                  <span />
                  <span />
                </div>
                <ul className="point-list">
                  {item.points.map((point) => (
                    <li key={point}>{point}</li>
                  ))}
                </ul>
              </article>
            ))}
          </div>
        </section>

        <section className="section section-tinted" id="workflow">
          <div className="section-heading reveal">
            <div className="eyebrow">Visible workflow</div>
            <h2>你看到的不只是结果，而是任务如何被完成</h2>
            <p>
              任务从发起到交付沿着明确链路推进。规划、检索、证据整理与写作全部作为步骤暴露给用户，而不是隐藏在一个模糊的 agent loop 里。
            </p>
          </div>

          <div className="workflow-rail reveal">
            {workflow.map((step, index) => (
              <div key={step} className="workflow-node">
                <div className="workflow-number">{index + 1}</div>
                <div className="workflow-card">
                  <strong>{step}</strong>
                  <span>
                    {index === 0 && '挂载语雀空间与内部资料'}
                    {index === 1 && '输入明确目标与交付要求'}
                    {index === 2 && '分类为 kb_only / web_only / hybrid'}
                    {index === 3 && '命中片段、摘要与关键上下文'}
                    {index === 4 && '引入近期外部来源补齐判断'}
                    {index === 5 && '清洗冲突、合并证据、形成结构'}
                    {index === 6 && '输出结构化报告与建议'}
                    {index === 7 && '保留 sources 与 artifacts 供回看'}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="section" id="preview">
          <div className="section-heading reveal">
            <div className="eyebrow">Product preview</div>
            <h2>围绕任务闭环设计，而不是堆砌功能噪音</h2>
            <p>
              界面重点放在工作台、Run 详情和知识准备三部分，让用户始终知道任务在哪一步、依据来自哪里、最终产出是什么。
            </p>
          </div>

          <div className="preview-grid">
            {previews.map((item, index) => (
              <article key={item.title} className={`preview-card reveal reveal-stagger-${index + 1}`}>
                <div className="preview-window">
                  <div className="preview-bar">
                    <span />
                    <span />
                    <span />
                  </div>
                  <div className="preview-canvas">
                    <div className="mini-nav" />
                    <div className="mini-layout">
                      <div className="mini-panel mini-panel-large" />
                      <div className="mini-column">
                        <div className="mini-panel mini-panel-mid" />
                        <div className="mini-panel mini-panel-small" />
                      </div>
                    </div>
                  </div>
                </div>
                <div className="preview-copy">
                  <div className="preview-eyebrow">{item.eyebrow}</div>
                  <h3>{item.title}</h3>
                  <p>{item.description}</p>
                  <div className="tag-row">
                    {item.tags.map((tag) => (
                      <span key={tag} className="tag">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="section section-tinted" id="scenarios">
          <div className="section-heading reveal">
            <div className="eyebrow">Use cases</div>
            <h2>适合真正需要知识和交付的工作</h2>
            <p>
              输入一个目标，系统去检索、整理和写作。重点不是多会聊天，而是最终能形成对业务有用的结构化输出。
            </p>
          </div>

          <div className="scenario-grid">
            {scenarios.map((item, index) => (
              <article key={item.title} className={`scenario-card reveal reveal-stagger-${index + 1}`}>
                <h3>{item.title}</h3>
                <p>{item.description}</p>
                <div className="tag-row">
                  {item.tags.map((tag) => (
                    <span key={tag} className="tag">
                      {tag}
                    </span>
                  ))}
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="section" id="differentiation">
          <div className="section-heading reveal">
            <div className="eyebrow">Why Knowvia</div>
            <h2>面向知识工作，而不是把一切都伪装成万能 AI</h2>
            <p>
              它把任务拆成可见步骤，把结论挂回来源，把技能沉淀成可复用层。产品价值在工作如何完成，而不是一句答案如何包装。
            </p>
          </div>

          <div className="compare-grid">
            {compareCards.map((item, index) => (
              <article
                key={item.title}
                className={`compare-card ${item.featured ? 'compare-card-featured' : ''} reveal reveal-stagger-${index + 1}`}
              >
                <h3>{item.title}</h3>
                <ul className="point-list">
                  {item.lines.map((line) => (
                    <li key={line}>{line}</li>
                  ))}
                </ul>
              </article>
            ))}
          </div>
        </section>

        <section className="section section-tinted" id="integrations">
          <div className="section-heading reveal">
            <div className="eyebrow">Integrations & trust</div>
            <h2>围绕真实能力构建，而不是装饰性 logo 墙</h2>
            <p>
              Knowvia 以语雀知识、Go API、Python RAG 与持久化执行链路为基础，服务于真正要完成研究和交付的流程。
            </p>
          </div>

          <div className="integration-panel reveal">
            <div className="integration-grid">
              {integrations.map((item) => (
                <div key={item} className="integration-tile">
                  {item}
                </div>
              ))}
            </div>
            <div className="trust-copy">
              <div className="trust-row">
                <strong>Grounded retrieval</strong>
                <span>内部语雀知识与外部网页证据并行整理</span>
              </div>
              <div className="trust-row">
                <strong>Durable runs</strong>
                <span>Run、steps、artifacts 与 sources 作为一等对象持久保存</span>
              </div>
              <div className="trust-row">
                <strong>Visible delivery</strong>
                <span>通过 SSE 事件流持续反馈执行状态与报告生成过程</span>
              </div>
            </div>
          </div>
        </section>

        <section className="section cta-section" id="cta">
          <div className="cta-card reveal">
            <div>
              <div className="eyebrow">Start with one run</div>
              <h2>把内部知识真正变成行动与交付</h2>
              <p>
                从知识连接、执行规划到结构化报告，Knowvia 帮你把复杂任务收束进同一条可追踪链路。
              </p>
            </div>
            <div className="cta-actions">
              <a className="button button-primary" href="#">
                立即体验
              </a>
              <a className="button button-secondary" href="#preview">
                查看工作台预览
              </a>
            </div>
          </div>
        </section>
      </main>

      <footer className="footer">
        <div>
          <div className="brand footer-brand">
            <span className="brand-mark">
              <img src={appLogo} alt="" className="brand-mark-image" />
            </span>
            <span>Knowvia</span>
          </div>
          <p>Grounded RAG, visible execution, reusable skills.</p>
        </div>
        <div className="footer-links">
          <a href="#capabilities">能力</a>
          <a href="#scenarios">场景</a>
          <a href="#integrations">集成</a>
          <a href="#cta">开始体验</a>
        </div>
      </footer>
    </div>
  );
}

export default App;
