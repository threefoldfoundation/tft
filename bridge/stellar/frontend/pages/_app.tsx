import App from 'next/app'
import Head from 'next/head'
import '../styles.css'

declare global {
  interface Window {
      $crisp: any;
      CRISP_WEBSITE_ID: string;
  }
}

export default class Root extends App {
  componentDidMount(): void {
    // Include the Crisp code here, without the <script></script> tags
    window.$crisp = [];
    window.CRISP_WEBSITE_ID = "1a5a5241-91cb-4a41-8323-5ba5ec574da0";

    (function() {
      var d = document;
      var s = d.createElement("script");

      s.src = "https://client.crisp.chat/l.js";
      s.async = true;
      d.getElementsByTagName("head")[0].appendChild(s);
    })();
  }

  render() {
    const { Component } = this.props

    return (
      <>
        <Head>
          <title>Threefold BSC Bridge</title>
        </Head>

        <Component />
      </>
    )
  }
}
