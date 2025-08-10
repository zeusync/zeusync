(function(){
  const canvas = document.getElementById('canvas');
  const ctx = canvas.getContext('2d');
  const statusEl = document.getElementById('status');
  const statsEl = document.getElementById('stats');
  const btn = document.getElementById('restartBtn');

  let ws;
  function connect(){
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    ws = new WebSocket(proto + '://' + location.host + '/ws');
    ws.onopen = () => { statusEl.textContent = 'WS: connected'; };
    ws.onclose = () => { statusEl.textContent = 'WS: closed, reconnect...'; setTimeout(connect, 1000); };
    ws.onmessage = (ev) => {
      const frame = JSON.parse(ev.data);
      render(frame);
    };
  }
  connect();

  btn.addEventListener('click', () => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send('restart');
    } else {
      fetch('/restart', {method:'POST'});
    }
  });

  const logEl = document.getElementById('log');

  function render(frame){
    const grid = frame.grid;
    const rows = grid.length; const cols = grid[0].length;
    const cell = Math.floor(Math.min(canvas.width/cols, canvas.height/rows));
    ctx.fillStyle = '#000'; ctx.fillRect(0,0,canvas.width,canvas.height);
    for (let y=0;y<rows;y++){
      for (let x=0;x<cols;x++){
        switch(grid[y][x]){
          case 'empty': ctx.fillStyle = '#101010'; break;
          case 'trap': ctx.fillStyle = '#8b0000'; break;
          case 'artifact': ctx.fillStyle = '#8a2be2'; break;
          case 'exit': ctx.fillStyle = '#006400'; break;
          case 'fog': ctx.fillStyle = '#2f4f4f'; break;
          default: ctx.fillStyle = '#101010';
        }
        ctx.fillRect(x*cell, y*cell, cell-1, cell-1);
      }
    }
    // robot
    ctx.fillStyle = '#ffff00';
    ctx.beginPath();
    ctx.arc(frame.pos_x*cell + cell/2, frame.pos_y*cell + cell/2, cell/3, 0, Math.PI*2);
    ctx.fill();

    const out = frame.outcome ? ` outcome=${frame.outcome}` : '';
    statsEl.textContent = `tick=${frame.tick} elapsed=${frame.elapsed_ticks} pos=(${frame.pos_x},${frame.pos_y}) hp=${frame.hp} energy=${frame.energy} exitVisible=${frame.exit_visible} trapsRemembered=${frame.remembered_traps}${out}`;

    if (Array.isArray(frame.events)) {
      for (const ev of frame.events) {
        const li = document.createElement('li');
        li.textContent = `t${frame.tick}: ${ev}`;
        logEl.insertBefore(li, logEl.firstChild);
      }
      // cap log length
      while (logEl.children.length > 50) logEl.removeChild(logEl.lastChild);
    }
  }
})();
