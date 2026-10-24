/* ==========================================================
   UNO MULTIPLAYER CLIENT LOGIC
   Pure Data-Driven UI connecting to Go Engine HTTP + WebSocket
   ========================================================== */

// --- Global Client State ---
let state = {
    playerId: null,
    playerName: "",
    roomId: null,
    isHost: false,
    ws: null,
    gameSnapshot: null,
    pendingWildCard: null
};

// --- Color & Kind Mapping Constants ---
const COLOR_NAMES = { 0: "NoColor", 1: "Red", 2: "Blue", 3: "Green", 4: "Yellow" };
const COLOR_CLASSES = { 1: "card-red", 2: "card-blue", 3: "card-green", 4: "card-yellow" };
const COLOR_HEX = { 1: "#ef4444", 2: "#3b82f6", 3: "#10b981", 4: "#f59e0b" };

const KIND_NAMES = {
    1: "Number",
    2: "Skip",
    3: "Reverse",
    4: "DrawTwo",
    5: "Wild",
    6: "WildDrawFour"
};

const SYMBOL_DISPLAY = {
    2: "⊘",   // Skip
    3: "⇄",   // Reverse
    4: "+2",  // Draw Two
    5: "★",   // Wild
    6: "+4"   // Wild Draw Four
};

// --- Web Audio Synthesizer (Instant 0-lag Sound FX) ---
const audioCtx = new (window.AudioContext || window.webkitAudioContext)();

function playSound(type) {
    if (audioCtx.state === 'suspended') audioCtx.resume();
    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();
    osc.connect(gain);
    gain.connect(audioCtx.destination);

    const now = audioCtx.currentTime;
    if (type === 'click') {
        osc.frequency.setValueAtTime(400, now);
        osc.frequency.exponentialRampToValueAtTime(200, now + 0.05);
        gain.gain.setValueAtTime(0.3, now);
        gain.gain.linearRampToValueAtTime(0.01, now + 0.05);
        osc.start(now);
        osc.stop(now + 0.05);
    } else if (type === 'card') {
        osc.frequency.setValueAtTime(300, now);
        osc.frequency.exponentialRampToValueAtTime(600, now + 0.08);
        gain.gain.setValueAtTime(0.25, now);
        gain.gain.linearRampToValueAtTime(0.01, now + 0.08);
        osc.start(now);
        osc.stop(now + 0.08);
    } else if (type === 'turn') {
        osc.type = 'triangle';
        osc.frequency.setValueAtTime(523.25, now); // C5
        osc.frequency.setValueAtTime(659.25, now + 0.1); // E5
        gain.gain.setValueAtTime(0.3, now);
        gain.gain.linearRampToValueAtTime(0.01, now + 0.25);
        osc.start(now);
        osc.stop(now + 0.25);
    } else if (type === 'error') {
        osc.type = 'sawtooth';
        osc.frequency.setValueAtTime(160, now);
        osc.frequency.exponentialRampToValueAtTime(90, now + 0.22);
        gain.gain.setValueAtTime(0.35, now);
        gain.gain.linearRampToValueAtTime(0.01, now + 0.22);
        osc.start(now);
        osc.stop(now + 0.22);
    }
}

// --- DOM References ---
const screens = {
    lobby: document.getElementById('lobbyScreen'),
    waiting: document.getElementById('waitingScreen'),
    game: document.getElementById('gameScreen')
};

const headerRoomInfo = document.getElementById('headerRoomInfo');
const roomCodeDisplay = document.getElementById('roomCodeDisplay');
const btnCopyInvite = document.getElementById('btnCopyInvite');

// Lobby Elements
const tabCreate = document.getElementById('tabCreate');
const tabJoin = document.getElementById('tabJoin');
const createRoomForm = document.getElementById('createRoomForm');
const joinRoomForm = document.getElementById('joinRoomForm');
const lobbyError = document.getElementById('lobbyError');

// Waiting Elements
const waitingRoomCode = document.getElementById('waitingRoomCode');
const waitingPlayerSlots = document.getElementById('waitingPlayerSlots');
const btnStartGame = document.getElementById('btnStartGame');
const waitingHostNotice = document.getElementById('waitingHostNotice');

// Game Elements
const opponentsArea = document.getElementById('opponentsArea');
const turnStatusBanner = document.getElementById('turnStatusBanner');
const turnStatusText = document.getElementById('turnStatusText');
const actionBanner = document.getElementById('actionBanner');
const actionBannerText = document.getElementById('actionBannerText');
const topCardContainer = document.getElementById('topCardContainer');
const drawPile = document.getElementById('drawPile');
const drawPileCountBadge = document.getElementById('drawPileCountBadge');
const activeColorDot = document.getElementById('activeColorDot');
const activeColorName = document.getElementById('activeColorName');
const myHandContainer = document.getElementById('myHandContainer');
const myCardCountBadge = document.getElementById('myCardCountBadge');
const btnDrawTurn = document.getElementById('btnDrawTurn');

let lastSeenAction = "";
let lastSeenColor = null;
let actionTimer = null;

// Modals
const colorPickerModal = document.getElementById('colorPickerModal');
const gameOverModal = document.getElementById('gameOverModal');
const btnReturnLobby = document.getElementById('btnReturnLobby');
const toastNotification = document.getElementById('toastNotification');

// ==========================================================
// NAVIGATION & SCREEN SWITCHING
// ==========================================================
function switchScreen(screenName) {
    Object.keys(screens).forEach(key => {
        screens[key].classList.remove('active');
        screens[key].classList.add('hidden');
    });
    screens[screenName].classList.remove('hidden');
    screens[screenName].classList.add('active');

    if (screenName !== 'lobby') {
        headerRoomInfo.classList.remove('hidden');
        roomCodeDisplay.textContent = state.roomId;
    } else {
        headerRoomInfo.classList.add('hidden');
    }
}

function showToast(message, isError = false) {
    toastNotification.textContent = message;
    toastNotification.style.borderColor = isError ? '#ef4444' : '#38bdf8';
    toastNotification.classList.remove('hidden');
    setTimeout(() => {
        toastNotification.classList.add('hidden');
    }, 3000);
}

// Check URL query parameters for auto-join link (e.g. ?room=K9XQ)
window.addEventListener('DOMContentLoaded', () => {
    const urlParams = new URLSearchParams(window.location.search);
    const roomParam = urlParams.get('room');
    if (roomParam) {
        tabJoin.click();
        document.getElementById('joinRoomCode').value = roomParam.toUpperCase();
    }
});

// ==========================================================
// LOBBY TABS & FORMS
// ==========================================================
tabCreate.addEventListener('click', () => {
    tabCreate.classList.add('active');
    tabJoin.classList.remove('active');
    createRoomForm.classList.add('active');
    joinRoomForm.classList.remove('active');
    lobbyError.classList.add('hidden');
});

tabJoin.addEventListener('click', () => {
    tabJoin.classList.add('active');
    tabCreate.classList.remove('active');
    joinRoomForm.classList.add('active');
    createRoomForm.classList.remove('active');
    lobbyError.classList.add('hidden');
});

// Handle Create Room (HTTP POST /api/rooms)
createRoomForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    playSound('click');
    const name = document.getElementById('createPlayerName').value.trim();
    if (!name) return;

    state.playerName = name;
    state.playerId = Math.floor(1000 + Math.random() * 9000); // 4-digit unique ID
    state.isHost = true;

    try {
        const res = await fetch('/api/rooms', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ hostName: name, hostId: state.playerId })
        });
        if (!res.ok) throw new Error(await res.text());
        const data = await res.json();
        state.roomId = data.roomId;

        enterRoom();
    } catch (err) {
        lobbyError.textContent = err.message || "Failed to create room";
        lobbyError.classList.remove('hidden');
    }
});

// Handle Join Room (HTTP POST /api/rooms/{id}/join)
joinRoomForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    playSound('click');
    const code = document.getElementById('joinRoomCode').value.trim().toUpperCase();
    const name = document.getElementById('joinPlayerName').value.trim();
    if (!code || !name) return;

    state.playerName = name;
    state.playerId = Math.floor(1000 + Math.random() * 9000);
    state.roomId = code;
    state.isHost = false;

    try {
        const res = await fetch(`/api/rooms/${code}/join`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ playerName: name, playerId: state.playerId })
        });
        if (!res.ok) throw new Error(await res.text());
        enterRoom();
    } catch (err) {
        lobbyError.textContent = err.message || "Failed to join room";
        lobbyError.classList.remove('hidden');
    }
});

// Copy Invite Link to Clipboard
btnCopyInvite.addEventListener('click', () => {
    playSound('click');
    const inviteUrl = `${window.location.origin}/?room=${state.roomId}`;
    navigator.clipboard.writeText(inviteUrl).then(() => {
        showToast("Invite link copied to clipboard! 📋");
    });
});

// ==========================================================
// WEBSOCKET REAL-TIME CONNECTION
// ==========================================================
function enterRoom() {
    waitingRoomCode.textContent = state.roomId;
    switchScreen('waiting');

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws?roomId=${state.roomId}&playerId=${state.playerId}`;

    state.ws = new WebSocket(wsUrl);

    state.ws.onopen = () => {
        console.log("WebSocket connected to room:", state.roomId);
    };

    state.ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            if (data.type === "ERROR") {
                playSound('error');
                showToast("⚠️ " + (data.message || "Invalid move!"), true);
                return;
            }
            handleGameStateUpdate(data);
        } catch (err) {
            console.error("Failed to parse game snapshot:", err);
        }
    };

    state.ws.onclose = () => {
        showToast("Disconnected from game server", true);
    };
}

// ==========================================================
// GAME STATE RENDERER
// ==========================================================
function handleGameStateUpdate(snapshot) {
    state.gameSnapshot = snapshot;

    // Check if game is in Waiting phase (turnPlayerId == -1)
    if (snapshot.turnPlayerId === -1) {
        renderWaitingLobby(snapshot);
    } else {
        renderGameArena(snapshot);
    }
}

// 1. Render Waiting Lobby
function renderWaitingLobby(snapshot) {
    switchScreen('waiting');
    waitingPlayerSlots.innerHTML = "";

    snapshot.players.forEach(player => {
        const slot = document.createElement('div');
        slot.className = "player-slot filled";
        const initial = player.playerName.charAt(0).toUpperCase();
        slot.innerHTML = `
            <div class="slot-avatar">${initial}</div>
            <div class="slot-name">${player.playerName} ${player.playerId === state.playerId ? '(You)' : ''}</div>
        `;
        waitingPlayerSlots.appendChild(slot);
    });

    // Fill remaining slots up to 4
    for (let i = snapshot.players.length; i < 4; i++) {
        const emptySlot = document.createElement('div');
        emptySlot.className = "player-slot";
        emptySlot.innerHTML = `
            <div class="slot-avatar" style="background: rgba(255,255,255,0.06); color: var(--text-muted);">?</div>
            <div class="slot-name" style="color: var(--text-muted);">Waiting...</div>
        `;
        waitingPlayerSlots.appendChild(emptySlot);
    }

    // Show "Start Game" button only for Host when >= 2 players
    if (state.isHost) {
        waitingHostNotice.classList.add('hidden');
        btnStartGame.classList.remove('hidden');
        btnStartGame.disabled = snapshot.players.length < 2;
        if (snapshot.players.length < 2) {
            btnStartGame.style.opacity = "0.5";
            btnStartGame.style.cursor = "not-allowed";
        } else {
            btnStartGame.style.opacity = "1";
            btnStartGame.style.cursor = "pointer";
        }
    } else {
        btnStartGame.classList.add('hidden');
        waitingHostNotice.classList.remove('hidden');
    }
}

// Host clicks "Start Game"
btnStartGame.addEventListener('click', () => {
    playSound('click');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({ action: "START_GAME" }));
    }
});

// 2. Render Live Game Arena
function renderGameArena(snapshot) {
    switchScreen('game');

    const isMyTurn = (snapshot.turnPlayerId === state.playerId);

    // Turn Banner
    if (isMyTurn) {
        turnStatusBanner.className = "turn-status-banner my-turn";
        turnStatusText.textContent = "★ YOUR TURN! Play a card or draw";
        playSound('turn');
    } else {
        const activePlayer = snapshot.players.find(p => p.playerId === snapshot.turnPlayerId);
        const name = activePlayer ? activePlayer.playerName : "Opponent";
        turnStatusBanner.className = "turn-status-banner";
        turnStatusText.textContent = `Waiting for ${name}'s turn...`;
    }

    // Render Opponents
    opponentsArea.innerHTML = "";
    snapshot.players.forEach(p => {
        if (p.playerId !== state.playerId) {
            const oppChip = document.createElement('div');
            oppChip.className = `opponent-chip ${p.playerId === snapshot.turnPlayerId ? 'turn-active' : ''}`;
            const initial = p.playerName.charAt(0).toUpperCase();
            oppChip.innerHTML = `
                <div class="opp-avatar">${initial}</div>
                <div class="opp-info">
                    <span class="opp-name">${p.playerName}</span>
                    <span class="opp-cards">🂠 ${p.handCardCount} cards</span>
                </div>
            `;
            opponentsArea.appendChild(oppChip);
        }
    });

    // Render Top Discard Card
    topCardContainer.innerHTML = "";
    if (snapshot.discardPile) {
        const cardElem = createCardElement(snapshot.discardPile, false);
        topCardContainer.appendChild(cardElem);
    }

    // Render Active Color
    const activeCol = snapshot.activeColor;
    activeColorDot.style.background = COLOR_HEX[activeCol] || '#ef4444';
    activeColorName.textContent = COLOR_NAMES[activeCol] || 'RED';

    // Render Draw Pile Count
    drawPileCountBadge.textContent = snapshot.drawPile || 0;

    // Render Player Hand
    const me = snapshot.players.find(p => p.playerId === state.playerId);
    myHandContainer.innerHTML = "";
    if (me && me.handCards) {
        myCardCountBadge.textContent = `${me.handCards.length} cards`;
        const totalCards = me.handCards.length;

        me.handCards.forEach((card, idx) => {
            const cardElem = createCardElement(card, true);

            // Natural fan-out angle
            const angle = (idx - (totalCards - 1) / 2) * 3;
            cardElem.style.transform = `rotate(${angle}deg)`;

            cardElem.addEventListener('click', () => onCardClicked(card));
            myHandContainer.appendChild(cardElem);
        });
    }

    // Check for Winner
    const winner = snapshot.players.find(p => p.handCardCount === 0);
    if (winner) {
        document.getElementById('winnerTitle').textContent = `${winner.playerName} Wins! 🎉`;
        document.getElementById('winnerSubtitle').textContent = winner.playerId === state.playerId ? "Congratulations, you won!" : "Better luck next time!";
        gameOverModal.classList.remove('hidden');
    }
}

// ==========================================================
// CARD COMPONENT BUILDER (AUTHENTIC UNO DESIGN)
// ==========================================================
function createCardElement(card, isInteractive = false) {
    const cardDiv = document.createElement('div');
    const colorClass = COLOR_CLASSES[card.color] || 'card-wild';
    cardDiv.className = `card ${colorClass}`;

    let symbol = "";
    if (card.kind === 1) { // Number
        symbol = card.value.toString();
    } else {
        symbol = SYMBOL_DISPLAY[card.kind] || "";
    }

    cardDiv.innerHTML = `
        <div class="card-corner corner-tl">${symbol}</div>
        <div class="card-oval">
            <span class="card-symbol-center">${symbol}</span>
        </div>
        <div class="card-corner corner-br">${symbol}</div>
    `;

    return cardDiv;
}

// ==========================================================
// PLAYER ACTIONS (PLAY CARD & DRAW CARD)
// ==========================================================
function onCardClicked(card) {
    if (!state.gameSnapshot || state.gameSnapshot.turnPlayerId !== state.playerId) {
        showToast("Not your turn!", true);
        return;
    }

    // Check if card is Wild (5) or Wild Draw 4 (6) -> Open Color Picker
    if (card.kind === 5 || card.kind === 6) {
        state.pendingWildCard = card;
        colorPickerModal.classList.remove('hidden');
        return;
    }

    // Standard card play
    playCardAction(card, card.color);
}

// Color picker buttons
document.querySelectorAll('.color-pick-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        playSound('click');
        const chosenColor = parseInt(btn.getAttribute('data-color'));
        colorPickerModal.classList.add('hidden');
        if (state.pendingWildCard) {
            playCardAction(state.pendingWildCard, chosenColor);
            state.pendingWildCard = null;
        }
    });
});

function playCardAction(card, chosenColor) {
    playSound('card');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({
            action: "PLAY_CARD",
            card: card,
            color: chosenColor
        }));
    }
}

// Draw Card Button Click
function triggerDraw() {
    if (!state.gameSnapshot || state.gameSnapshot.turnPlayerId !== state.playerId) {
        showToast("Not your turn!", true);
        return;
    }
    playSound('card');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({ action: "DRAW_CARD" }));
    }
}

drawPile.addEventListener('click', triggerDraw);
btnDrawTurn.addEventListener('click', triggerDraw);

btnReturnLobby.addEventListener('click', () => {
    window.location.href = "/";
});
