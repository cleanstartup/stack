import type { Config } from '@stencil/core';

export const config: Config = {
  namespace: 'stack',
  srcDir: '/Users/adrian/git/cleanstartup/company/artifacts/stack/cmd/site/.stack/stencil-cache/src/assets/js',
  outputTargets: [
    {
      type: 'dist',
      dir: '/Users/adrian/git/cleanstartup/company/artifacts/stack/cmd/site/.stack/stencil-cache/dist',
      esmLoaderPath: '../loader',
    },
  ],
};
