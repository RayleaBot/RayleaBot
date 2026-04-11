export const appTheme = {
  token: {
    colorPrimary: '#0f6f70',
    colorInfo: '#0f6f70',
    colorSuccess: '#197b59',
    colorWarning: '#b26f16',
    colorError: '#b2432f',
    colorText: '#162127',
    colorTextSecondary: '#60717a',
    colorBgBase: '#eef1ec',
    colorBgContainer: 'rgba(255, 255, 255, 0.94)',
    colorBorder: 'rgba(35, 48, 56, 0.12)',
    borderRadius: 18,
    wireframe: false,
    fontFamily: '"PingFang SC", "Hiragino Sans GB", "Source Han Sans SC", "Microsoft YaHei", sans-serif',
  },
  components: {
    Layout: {
      siderBg: 'transparent',
      headerBg: 'transparent',
      bodyBg: 'transparent',
      triggerBg: '#11181d',
    },
    Card: {
      headerBg: 'transparent',
    },
    Button: {
      borderRadius: 14,
      controlHeight: 40,
    },
    Input: {
      controlHeight: 40,
    },
    Select: {
      controlHeight: 40,
    },
  },
} as const
