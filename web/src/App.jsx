import React from 'react';
import batteryIcon from './assets/ios-battery.svg';
import cellularIcon from './assets/ios-cellular.svg';
import appLogo from './assets/logo.png';
import wifiIcon from './assets/ios-wifi.svg';

const navItems = [
  ['平台能力', '#capabilities'],
  ['工作流', '#workflow'],
  ['产品模块', '#product'],
  ['场景', '#scenarios'],
];

const proof = [
  ['知识驱动', '连接企业文档知识库，回答和任务都基于可信上下文。'],
  ['执行可见', 'Run 时间线展示规划、检索、合并、写作和交付状态。'],
  ['结果可追溯', '最终报告保留来源卡、片段命中和证据映射。'],
];

const problems = [
  ['知识分散', '企业文档散落在不同平台，团队很难把已有沉淀转成可执行判断。'],
  ['Agent 黑箱', '普通聊天式 AI 看不到执行步骤，也难以判断结论是否可靠。'],
  ['交付难复查', '报告写完后很难回到来源证据，协作审阅成本高。'],
];

const capabilities = [
  ['知识库连接与同步', '接入语雀、飞书和内部资料源，持续同步目录、正文、片段索引和更新状态。'],
  ['聊天问答', '围绕已挂载知识库回答问题，保留命中片段、上下文范围和来源路径。'],
  ['Skills 调用', '将研究报告、行动计划、摘要整理等能力封装成可复用执行策略。'],
  ['Run 任务创建', '把复杂问题建模为 Run，拆解计划、检索、证据合并、报告写作和最终交付。'],
  ['执行时间线', '每一步都有状态、摘要和产物指向，让长任务不再是黑箱等待。'],
  ['结构化报告与来源追溯', '最终回答、报告阶段和来源卡同时保存，方便审阅、复查和复用。'],
];

const workflow = [
  ['01', '连接知识源', '挂载语雀、飞书或内部资料，建立可检索上下文。'],
  ['02', '提出任务目标', '选择知识范围、模型和 Skill，明确交付要求。'],
  ['03', '规划 Run', '系统拆解执行步骤并判断任务模式。'],
  ['04', '检索与合并证据', '命中内部片段，整理来源卡和证据链。'],
  ['05', '生成结构化报告', '输出最终回答、报告正文和行动建议。'],
  ['06', '回溯来源', '从结论返回文档片段，支持审阅和团队复用。'],
];

const modules = [
  {
    title: '统一任务入口',
    body: '在同一工作台中完成提问、知识库挂载、Skill 选择和 Run 创建。',
    points: ['聊天问答', '知识范围', 'Skill 选择'],
  },
  {
    title: '可见执行过程',
    body: '将复杂任务拆解为时间线步骤，持续展示规划、检索、证据合并和写作状态。',
    points: ['步骤状态', '过程摘要', '阶段产物'],
  },
  {
    title: '企业知识连接',
    body: '管理语雀、飞书和内部资料源的同步状态，保持可检索上下文持续更新。',
    points: ['同步状态', '文档规模', '异常提示'],
  },
  {
    title: '可复用执行能力',
    body: '把研究报告、行动计划、竞品整理等团队方法沉淀为可调用 Skills。',
    points: ['启用开关', '导入复用', '模式管理'],
  },
];

const scenarios = [
  ['研究分析', '把内部沉淀、外部材料和历史报告整理为可追溯研究结论。'],
  ['增长复盘', '从实验记录和业务文档中提取问题、证据、优先级和行动计划。'],
  ['知识问答', '围绕企业文档提问，降低重复沟通和人工查找成本。'],
  ['客户支持', '将 FAQ、产品说明和历史案例转成可复查的回答依据。'],
];

const runSteps = [
  ['Planning', '已完成', '识别目标、交付格式和知识范围。'],
  ['Yuque Search', '已完成', '命中 18 个内部知识片段。'],
  ['Evidence Merge', '运行中', '正在构建报告证据链。'],
];

function IosStatusBar() {
  return (
    <div className="phone-status" aria-label="iOS 状态栏">
      <div className="phone-time">9:41</div>
      <div className="phone-system-icons" aria-hidden="true">
        <img className="phone-cellular" src={cellularIcon} alt="" />
        <img className="phone-wifi" src={wifiIcon} alt="" />
        <img className="phone-battery" src={batteryIcon} alt="" />
      </div>
    </div>
  );
}

function ProductPreview() {
  return (
    <div className="phone-showcase" aria-label="Knowvia Agent 手机界面预览">
      <article className="phone-page">
        <IosStatusBar />
        <div className="phone-nav">
          <button type="button" aria-label="菜单">☰</button>
          <strong>Gemma 3n E4B</strong>
          <span>T=0.05</span>
        </div>
        <div className="phone-body phone-chat-body">
          <section className="chat-bubble chat-bubble-assistant">
            <strong>Knowvia · Lite</strong>
            <p>可以基于知识库回答，也可以创建 Run 执行更长的研究任务。</p>
          </section>
          <section className="chat-bubble chat-bubble-user">
            <p>基于增长复盘空间，整理 Q2 漏斗问题并输出下周行动计划。</p>
          </section>
          <section className="chat-bubble chat-bubble-assistant">
            <strong>Knowvia · Lite</strong>
            <p>已挂载增长复盘空间，并调用结构化研究报告 Skill。将先检索内部片段，再生成报告草稿与来源映射。</p>
            <div className="phone-tags">
              <em>结构化研究报告</em>
              <em>增长复盘空间</em>
            </div>
          </section>
        </div>
        <div className="phone-composer">
          <span>输入任务或问题...</span>
          <button type="button" aria-label="发送">↑</button>
        </div>
        <div className="phone-safe-bottom" aria-hidden="true">
          <span />
        </div>
      </article>

      <article className="phone-page">
        <IosStatusBar />
        <div className="phone-title">
          <span>运行中</span>
          <h3>Q2 增长行动建议</h3>
          <p>整理漏斗问题、证据来源和下周行动建议。</p>
        </div>
        <div className="phone-body">
          <section className="phone-timeline">
            {runSteps.map(([title, status, body]) => (
              <div key={title}>
                <i />
                <div>
                  <strong>{title}</strong>
                  <p>{body}</p>
                </div>
                <em>{status}</em>
              </div>
            ))}
          </section>
          <section className="phone-card">
            <span>YUQUE · 0.92</span>
            <strong>Q2 增长复盘 / Onboarding 漏斗</strong>
            <p>新用户在第二步转化下滑明显，应缩短引导路径并强化模板入口。</p>
          </section>
        </div>
        <div className="phone-safe-bottom" aria-hidden="true">
          <span />
        </div>
      </article>
    </div>
  );
}

function SectionIntro({ eyebrow, title, children }) {
  const renderedTitle = Array.isArray(title)
    ? title.map((line) => (
      <span className="title-line" key={line}>{line}</span>
    ))
    : title;

  return (
    <div className="section-intro">
      <span>{eyebrow}</span>
      <h2>{renderedTitle}</h2>
      <p>{children}</p>
    </div>
  );
}

function App() {
  return (
    <div className="site-shell">
      <header className="site-header">
        <a className="brand" href="#top" aria-label="Knowvia Agent 首页">
          <img src={appLogo} alt="" />
          <span>Knowvia Agent</span>
        </a>
        <nav aria-label="主导航">
          {navItems.map(([label, href]) => (
            <a href={href} key={href}>{label}</a>
          ))}
        </nav>
      </header>

      <main id="top">
        <section className="hero">
          <div className="hero-copy">
            <span>企业知识执行工作台</span>
            <h1>让企业知识进入可执行工作流</h1>
            <p>
              Knowvia Agent 连接企业文档知识库，把提问、检索、任务执行、报告生成和来源追溯整合到同一个工作台，让团队从知识沉淀走向可验证的交付结果。
            </p>
            <div className="hero-actions">
              <a className="primary-link" href="#product">查看产品能力</a>
              <a className="secondary-link" href="#workflow">了解执行链路</a>
            </div>
          </div>
          <ProductPreview />
        </section>

        <section className="proof-row" aria-label="产品原则">
          {proof.map(([title, body]) => (
            <article key={title}>
              <strong>{title}</strong>
              <p>{body}</p>
            </article>
          ))}
        </section>

        <section className="section problem-section">
          <SectionIntro
            eyebrow="Why Knowvia"
            title={['企业知识工作不缺 AI，', '缺的是可信执行']}
          >
            真正困难的不是生成一段回答，而是把知识、步骤、证据和交付连接起来。
          </SectionIntro>
          <div className="problem-grid">
            {problems.map(([title, body]) => (
              <article key={title}>
                <h3>{title}</h3>
                <p>{body}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="section" id="capabilities">
          <SectionIntro eyebrow="平台能力" title={['围绕知识、执行和证据', '建立能力闭环']}>
            从知识连接到报告交付，Knowvia Agent 将企业知识组织成可执行、可复查、可持续复用的工作流。
          </SectionIntro>
          <div className="capability-grid">
            {capabilities.map(([title, body]) => (
              <article key={title}>
                <h3>{title}</h3>
                <p>{body}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="section workflow-section" id="workflow">
          <SectionIntro eyebrow="Workflow" title={['从问题到报告，', '执行过程清晰可见']}>
            Run 将复杂任务拆成连续步骤，让团队知道任务正在做什么、依据是什么、结果如何形成。
          </SectionIntro>
          <div className="workflow-list">
            {workflow.map(([number, title, body]) => (
              <article key={title}>
                <span>{number}</span>
                <div>
                  <strong>{title}</strong>
                  <p>{body}</p>
                </div>
              </article>
            ))}
          </div>
        </section>

        <section className="section product-section" id="product">
          <SectionIntro eyebrow="Product Suite" title={['从知识准备到结果交付，', '形成一体化工作台']}>
            Knowvia Agent 将企业知识、执行策略、任务过程和交付结果组织在同一个产品体系中，减少跨工具切换，让团队围绕同一份证据协作。
          </SectionIntro>
          <div className="suite-layout">
            <article className="suite-preview">
              <div className="suite-preview-head">
                <span>Active Run</span>
                <strong>Q2 增长行动建议</strong>
              </div>
              <div className="suite-preview-body">
                <div className="suite-task">
                  <span>任务目标</span>
                  <p>基于增长复盘空间，整理漏斗问题、证据来源和下周行动建议。</p>
                </div>
                <div className="suite-steps">
                  {runSteps.map(([title, status]) => (
                    <div key={title}>
                      <i />
                      <strong>{title}</strong>
                      <span>{status}</span>
                    </div>
                  ))}
                </div>
                <div className="suite-evidence">
                  <span>来源证据</span>
                  <strong>Q2 增长复盘 / Onboarding 漏斗</strong>
                  <p>新用户在第二步转化下滑明显，应缩短引导路径并强化模板入口。</p>
                </div>
              </div>
            </article>

            <div className="suite-cards">
              {modules.map((item, index) => (
                <article className="suite-card" key={item.title}>
                  <span>{String(index + 1).padStart(2, '0')}</span>
                  <h3>{item.title}</h3>
                  <p>{item.body}</p>
                  <div>
                    {item.points.map((point) => (
                      <em key={point}>{point}</em>
                    ))}
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="section" id="scenarios">
          <SectionIntro eyebrow="Use Cases" title={['适合需要可信交付的', '企业知识场景']}>
            当团队需要基于内部知识完成研究、复盘、支持和决策，Knowvia Agent 能把过程和结果一并交付。
          </SectionIntro>
          <div className="scenario-grid">
            {scenarios.map(([title, body]) => (
              <article key={title}>
                <h3>{title}</h3>
                <p>{body}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="cta-section">
          <span>Knowvia Agent</span>
          <h2>
            <span className="title-line">让企业知识进入可执行、</span>
            <span className="title-line">可验证、可复用的工作流</span>
          </h2>
          <a className="primary-link" href="#top">回到顶部</a>
        </section>
      </main>
    </div>
  );
}

export default App;
