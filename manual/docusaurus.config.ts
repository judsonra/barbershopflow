import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Manual do BarberFlow',
  tagline: 'Como usar o BarberFlow no dia a dia da barbearia',
  // O manual ainda não tem endereço público: troque ao definir a publicação.
  url: 'https://manual.barberflow.local',
  baseUrl: '/',
  onBrokenLinks: 'throw',
  i18n: {
    defaultLocale: 'pt-BR',
    locales: ['pt-BR'],
  },
  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
        },
        blog: false,
      } satisfies Preset.Options,
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'Manual do BarberFlow',
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
