const html = require('choo/html');
const find = require('lodash/find');
const contact = require('./contact.js');

module.exports = (state, prev, send) => {
  const issue = find(state.issues, ['id', state.activeIssue]);
  const contactsLeft = issue.contacts.length - (state.contactIndex + 1);

  const contactsLeftText = contactsLeft > 0 ? contactsLeft + " calls left" : "This is the last contact";

  // console.log(issue);

  function outcome() {
    send('callComplete');
  }

  return html`
    <section class="call">
      <header class="call__header">
        <h2 class="call__title">${issue.name}</h2>
        <h3 class="call__reason">${issue.reason}</h2>
      </header>

      ${contact(state, prev, send)}

      <div class="call__script">
        <h3 class="call__script__header">Your script:</h3>
        <p class="call__script__body">${issue.script}</p>
      </div>

      <menu class="call__outcomes">
        <menuitem onclick=${outcome}>Unavailable</menuitem>
        <menuitem onclick=${outcome}>Left Voicemail</menuitem>
        <menuitem onclick=${outcome}>Made Contact</menuitem>
      </menu>

      <div class="call__promote">
        <p>${contactsLeftText} for this issue • <a href="#">Tweet this issue</a></p>
      </div>
    </section>
  `;
}
