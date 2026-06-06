import { Component, State, h } from '@stencil/core';

@Component({
  tag: 'demo-card',
  shadow: false,
})
export class DemoCard {
  @State() clicked = false;

  private handleClick = () => {
    this.clicked = true;
    console.log('hello from way2go');
  };

  render() {
    return (
      <section class="card">
        <h1 class="title">hello from way2go</h1>
        <p class="lead">This page loads global styles and Stencil components.</p>
        <p class="hint">{this.clicked ? 'clicked!' : 'Open the console and click the button.'}</p>
        <button class="button" onClick={this.handleClick}>Click me</button>
      </section>
    );
  }
}
