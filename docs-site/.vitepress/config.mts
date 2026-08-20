import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Gira',
  description: 'Jira-style project flow on GitHub',
  cleanUrls: true,
  sitemap: {
    hostname: 'https://gira.statpan.com'
  },
  themeConfig: {
    logo: { text: 'Gira' },
    search: {
      provider: 'local'
    },
    nav: [
      {
        text: 'Start',
        items: [
          { text: 'Install', link: '/install' },
          { text: 'Quick Start', link: '/quickstart' },
          { text: 'Global Config', link: '/global-config' },
          { text: 'Troubleshooting', link: '/troubleshooting' }
        ]
      },
      {
        text: 'Daily Delivery',
        items: [
          { text: 'Ticket Workflow', link: '/ticket-workflow' },
          { text: 'Branch Behavior', link: '/branch-policy' },
          { text: 'Readiness And Audit', link: '/readiness-audit' }
        ]
      },
      {
        text: 'Agents',
        items: [
          { text: 'Agent Operator Golden Path', link: '/agent-operator-skill' },
          { text: 'Agent Handoff Queue', link: '/agent-handoff-queue' },
          { text: 'Agent Delegation Lanes', link: '/agent-delegation-lanes' },
          { text: 'Worker Boundary', link: '/worker-boundary' }
        ]
      },
      {
        text: 'Advanced Orchestration',
        items: [
          { text: 'Workspace', link: '/workspace' },
          { text: 'Goal Mode', link: '/goal-mode' },
          { text: 'Sprint And Release', link: '/sprint-release' },
          { text: 'PM Skill', link: '/pm-skill' },
          { text: 'Feature Map', link: '/feature-map' },
          { text: 'Jira Mapping', link: '/jira-mapping' },
          { text: 'Jira-Primary Provider', link: '/jira-primary-provider' }
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'Command Reference (generated)', link: '/command-reference' },
          { text: 'Command Capabilities (generated)', link: '/command-capabilities' },
          { text: 'Distribution', link: '/distribution' },
          { text: 'Adoption Signals', link: '/adoption-signals' },
          { text: 'State Model', link: '/state-model' },
          { text: 'Command Surface', link: '/command-surface' },
          { text: 'API Limits', link: '/api-limits' },
          { text: 'Cost Profiles', link: '/cost-profiles' },
          { text: 'Gira 2.0 Control Plane', link: '/v2-control-plane' },
          { text: 'Gira 3.0 Local Report Bundle', link: '/gira-3-local-report-bundle' },
          { text: 'Hosted Control Plane', link: '/hosted-control-plane' },
          { text: 'Workspace Dashboard Gaps', link: '/workspace-dashboard-contract-gaps' },
          { text: 'CLI Surface Diagnostic', link: '/cli-surface-diagnostic' },
          { text: 'CLI Workflow Complexity', link: '/cli-workflow-complexity' },
          { text: 'Workflow Efficiency KPIs', link: '/workflow-efficiency-kpis' },
          { text: 'Closure Funnel Stats', link: '/closure-funnel-stats' },
          { text: 'Task Momentum Loop', link: '/task-momentum-loop' }
        ]
      },
      { text: 'GitHub', link: 'https://github.com/StatPan/gira' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'Install', link: '/install' },
          { text: 'Quick Start', link: '/quickstart' },
          { text: 'Global Config', link: '/global-config' },
          { text: 'Troubleshooting', link: '/troubleshooting' }
        ]
      },
      {
        text: 'Daily Delivery',
        items: [
          { text: 'Ticket Workflow', link: '/ticket-workflow' },
          { text: 'Branch Behavior', link: '/branch-policy' },
          { text: 'Readiness And Audit', link: '/readiness-audit' }
        ]
      },
      {
        text: 'Agents',
        items: [
          { text: 'Agent Operator Golden Path', link: '/agent-operator-skill' },
          { text: 'Agent Handoff Queue', link: '/agent-handoff-queue' },
          { text: 'Agent Delegation Lanes', link: '/agent-delegation-lanes' },
          { text: 'Worker Boundary', link: '/worker-boundary' }
        ]
      },
      {
        text: 'Advanced Orchestration',
        items: [
          { text: 'Workspace', link: '/workspace' },
          { text: 'Goal Mode', link: '/goal-mode' },
          { text: 'Sprint And Release', link: '/sprint-release' },
          { text: 'PM Skill', link: '/pm-skill' },
          { text: 'Feature Map', link: '/feature-map' },
          { text: 'Jira Mapping', link: '/jira-mapping' },
          { text: 'Jira-Primary Provider', link: '/jira-primary-provider' }
        ]
      },
      {
        text: 'Reference',
        collapsed: true,
        items: [
          { text: 'Command Reference (generated)', link: '/command-reference' },
          { text: 'Command Capabilities (generated)', link: '/command-capabilities' },
          { text: 'Distribution', link: '/distribution' },
          { text: 'Adoption Signals', link: '/adoption-signals' },
          { text: 'State Model', link: '/state-model' },
          { text: 'Command Surface', link: '/command-surface' },
          { text: 'API Limits', link: '/api-limits' },
          { text: 'Cost Profiles', link: '/cost-profiles' },
          { text: 'Gira 2.0 Control Plane', link: '/v2-control-plane' },
          { text: 'Gira 3.0 Local Report Bundle', link: '/gira-3-local-report-bundle' },
          { text: 'Hosted Control Plane', link: '/hosted-control-plane' },
          { text: 'Workspace Dashboard Gaps', link: '/workspace-dashboard-contract-gaps' },
          { text: 'CLI Surface Diagnostic', link: '/cli-surface-diagnostic' },
          { text: 'CLI Workflow Complexity', link: '/cli-workflow-complexity' },
          { text: 'Workflow Efficiency KPIs', link: '/workflow-efficiency-kpis' },
          { text: 'Closure Funnel Stats', link: '/closure-funnel-stats' },
          { text: 'Task Momentum Loop', link: '/task-momentum-loop' }
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/StatPan/gira' }
    ],
    footer: {
      message: 'GitHub remains the source of truth.',
      copyright: 'MIT Licensed'
    }
  }
})
