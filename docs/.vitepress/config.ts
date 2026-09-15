import { defineConfig } from "vitepress";

const [owner, repoName] = (process.env.GITHUB_REPOSITORY ?? "adamsiwiec1/runhug").split("/");
const repo = `https://github.com/${owner}/${repoName}`;
const base = `/${repoName}/`;

export default defineConfig({
  title: "runhug",
  titleTemplate: ":title · runhug",
  description:
    "Find the best Hugging Face model. Deploy it on Runpod in minutes. Run it for pennies.",
  lang: "en-US",
  base,
  cleanUrls: true,
  lastUpdated: true,
  ignoreDeadLinks: false,
  sitemap: {
    hostname: `https://${owner}.github.io/${repoName}/`,
  },

  themeConfig: {
    nav: [
      { text: "Getting started", link: "/guide/getting-started", activeMatch: "/guide/getting-started" },
      { text: "Guides", link: "/guide/search", activeMatch: "/guide/" },
      { text: "Reference", link: "/reference/cli", activeMatch: "/reference/" },
      { text: "GitHub", link: repo },
    ],

    sidebar: {
      "/guide/": [
        {
          text: "Overview",
          items: [{ text: "What runhug is", link: "/guide/" }],
        },
        {
          text: "Getting started",
          items: [{ text: "Install & first deploy", link: "/guide/getting-started" }],
        },
        {
          text: "Guides",
          items: [
            { text: "Search & recommend", link: "/guide/search" },
            { text: "RunPod deploy", link: "/guide/runpod" },
            { text: "Local & HF connect", link: "/guide/local" },
            { text: "Index packs", link: "/guide/index-packs" },
          ],
        },
        {
          text: "More",
          items: [
            { text: "Troubleshooting", link: "/guide/troubleshooting" },
            { text: "Why RunPod-only", link: "/guide/runpod-only" },
          ],
        },
      ],
      "/reference/": [
        {
          text: "Reference",
          items: [
            { text: "CLI map", link: "/reference/cli" },
            { text: "Changelog", link: "/reference/changelog" },
            { text: "Community health", link: "/reference/community-health" },
          ],
        },
      ],
    },

    socialLinks: [{ icon: "github", link: repo }],

    editLink: {
      pattern: `${repo}/edit/main/docs/:path`,
      text: "Edit this page on GitHub",
    },

    search: { provider: "local" },

    footer: {
      message: "MIT licensed. Docs via VitePress + GitHub Pages.",
      copyright: "© 2026 Adam Siwiec",
    },
  },
});
