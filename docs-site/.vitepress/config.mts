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
      { text: 'Quick Start', link: '/quickstart' },
      { text: 'Gira 2.0', link: '/v2-control-plane' },
      { text: 'Gira 3.0', link: '/gira-3-local-report-bundle' },
      { text: 'Workspace', link: '/workspace' },
      { text: 'Agent Queue', link: '/agent-handoff-queue' },
      { text: 'Workflow', link: '/ticket-workflow' },
      { text: 'Goal Mode', link: '/goal-mode' },
      { text: 'Readiness', link: '/readiness-audit' },
      { text: 'Jira Provider', link: '/jira-primary-provider' },
      { text: 'Install', link: '/install' },
      { text: 'GitHub', link: 'https://github.com/StatPan/gira' }
    ],
    sidebar: [
      {
        text: 'Start',
        items: [
          { text: 'Install', link: '/install' },
          { text: 'Quick Start', link: '/quickstart' },
          { text: 'Distribution', link: '/distribution' },
          { text: 'Adoption Signals', link: '/adoption-signals' },
          { text: 'Troubleshooting', link: '/troubleshooting' }
        ]
      },
      {
        text: 'Configure',
        items: [
          { text: 'Global Config', link: '/global-config' },
          { text: 'Branch Policy', link: '/branch-policy' },
          { text: 'Jira To GitHub Mapping', link: '/jira-mapping' },
          { text: 'Jira-Primary Provider', link: '/jira-primary-provider' }
        ]
      },
      {
        text: 'Operate',
        items: [
          { text: 'Workspace', link: '/workspace' },
          { text: 'Agent Handoff Queue', link: '/agent-handoff-queue' },
          { text: 'Ticket Workflow', link: '/ticket-workflow' },
          { text: 'Feature Map', link: '/feature-map' },
          { text: 'Gira 2.0 Control Plane', link: '/v2-control-plane' },
          { text: 'Gira 3.0 Local Report Bundle', link: '/gira-3-local-report-bundle' },
          { text: 'Workspace Dashboard Gaps', link: '/workspace-dashboard-contract-gaps' },
          { text: 'State Model', link: '/state-model' },
          { text: 'API Limits', link: '/api-limits' },
          { text: 'Cost Profiles', link: '/cost-profiles' },
          { text: 'Command Surface', link: '/command-surface' },
          { text: 'CLI Surface Diagnostic', link: '/cli-surface-diagnostic' },
          { text: 'Goal Mode', link: '/goal-mode' },
          { text: 'Sprint And Release', link: '/sprint-release' },
          { text: 'Readiness And Audit', link: '/readiness-audit' },
          { text: 'Closure Funnel Stats', link: '/closure-funnel-stats' },
          { text: 'Task Momentum Loop', link: '/task-momentum-loop' }
        ]
      },
      {
        text: 'Agents And Contracts',
        items: [
          { text: 'Worker Boundary', link: '/worker-boundary' },
          { text: 'Agent Operator Skill', link: '/agent-operator-skill' },
          { text: 'Agent Delegation Lanes', link: '/agent-delegation-lanes' },
          { text: 'Hosted Control Plane', link: '/hosted-control-plane' }
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'Command Reference', link: '/command-reference' },
          { text: 'Command Capabilities', link: '/command-capabilities' }
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
