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
      { text: 'Guide', link: '/quickstart' },
      { text: 'Mapping', link: '/jira-mapping' },
      { text: 'Jira Provider', link: '/jira-primary-provider' },
      { text: 'Workflow', link: '/ticket-workflow' },
      { text: 'Branch Policy', link: '/branch-policy' },
      { text: 'Config', link: '/global-config' },
      { text: 'Distribution', link: '/distribution' },
      { text: 'GitHub', link: 'https://github.com/StatPan/gira' }
    ],
    sidebar: [
      {
        text: 'Get Started',
        items: [
          { text: 'Install', link: '/install' },
          { text: 'Quick Start', link: '/quickstart' },
          { text: 'Global Config', link: '/global-config' },
          { text: 'Jira-Primary Provider', link: '/jira-primary-provider' },
          { text: 'Ticket Workflow', link: '/ticket-workflow' },
          { text: 'Worker Boundary', link: '/worker-boundary' },
          { text: 'Branch Policy', link: '/branch-policy' },
          { text: 'Closure Funnel Stats', link: '/closure-funnel-stats' },
          { text: 'Command Reference', link: '/command-reference' },
          { text: 'Agent Operator Skill', link: '/agent-operator-skill' },
          { text: 'Agent Delegation Lanes', link: '/agent-delegation-lanes' }
        ]
      },
      {
        text: 'Operate',
        items: [
          { text: 'Sprint And Release', link: '/sprint-release' },
          { text: 'Jira To GitHub Mapping', link: '/jira-mapping' },
          { text: 'Jira-Primary Provider', link: '/jira-primary-provider' },
          { text: 'Distribution', link: '/distribution' },
          { text: 'Troubleshooting', link: '/troubleshooting' }
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
