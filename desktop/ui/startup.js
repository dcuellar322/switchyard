window.switchyardStartupError = (detail) => {
  document.querySelector('#status').textContent = 'Switchyard could not start safely.';
  const output = document.querySelector('#detail');
  output.textContent = detail;
  output.hidden = false;
};
