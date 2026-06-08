import { Component, State, h } from '@stencil/core';
import { DemoCopy } from './demo-copy';

@Component({
  tag: 'site-demo-card',
  shadow: false,
})
export class DemoCard {
  @State() clicked = false;

  private handleClick = () => {
    this.clicked = true;
  };

  render() {
    return (
      <section class="card">
        <h1 class="title">{DemoCopy.headline()}</h1>
        <p class="lead">{DemoCopy.intro()}</p>
        <p class="hint">{DemoCopy.hint(this.clicked)}</p>
        <button class="button" onClick={this.handleClick}>Click me</button>
      </section>
    );
  }
}

