const termOptions = {
    fontSize: 13,
        fontFamily: 'terminal,monospace',
        macOptionClickForcesSelection: true,
        macOptionIsMeta: true,
    theme: {
        foreground: '#d4d4d4',
        background: '#000000',
        cursor: '#adadad',
        black: '#000000',
        red: '#d81e00',
        green: '#5ea702',
        yellow: '#cfae00',
        blue: '#427ab3',
        magenta: '#89658e',
        cyan: '#00a7aa',
        white: '#dbded8',
        brightBlack: '#686a66',
        brightRed: '#f54235',
        brightGreen: '#99e343',
        brightYellow: '#fdeb61',
        brightBlue: '#84b0d8',
        brightMagenta: '#bc94b7',
        brightCyan: '#37e6e8',
        brightWhite: '#f1f1f0',
    }
};

const terminal = new Terminal(termOptions);
terminal.open(document.getElementById('terminal'));

const fitAddon = new FitAddon.FitAddon();
const ws = new WebSocket('ws://localhost:8080/ws');
const attachAddon = new AttachAddon.AttachAddon(ws);
    
terminal.loadAddon(fitAddon);
terminal.loadAddon(attachAddon);
fitAddon.fit();

const cols = 1024; // TODO: calculate font size and use that to calculate cols and rows 80x24
const rows = 768;
terminal.resize(cols, rows);

window.addEventListener('resize', () => {
    fitAddon.fit();
    terminal.resize(cols, rows);
});

// TODO: add zmodem addon support. Old school but cool. :D