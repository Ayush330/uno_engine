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
    pendingWildCard: null,
    lastClickedCardElement: null
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
const btnPassTurn = document.getElementById('btnPassTurn');
const btnSayUno = document.getElementById('btnSayUno');
const turnTimerBadge = document.getElementById('turnTimerBadge');
const turnTimerBar = document.getElementById('turnTimerBar');
const drawPileHint = document.getElementById('drawPileHint');
const tableSurface = document.getElementById('tableSurface');
const directionRing = document.getElementById('directionRing');
const tableShockwave = document.getElementById('tableShockwave');
const confettiCanvas = document.getElementById('confettiCanvas');

let lastSeenAction = "";
let lastSeenColor = null;
let actionTimer = null;
let turnTimerInterval = null;
let confettiAnimationId = null;

// Modals
const colorPickerModal = document.getElementById('colorPickerModal');
const gameOverModal = document.getElementById('gameOverModal');
const btnReturnLobby = document.getElementById('btnReturnLobby');
const toastNotification = document.getElementById('toastNotification');

// ==========================================================
// TABLETOP INTERACTION & FX HELPERS
// ==========================================================

// Check if card matches discard pile or active color (exact mirror of Go domain)
function isCardPlayable(card, topDiscard, activeColor) {
    if (!card) return false;
    // Wild cards (5: Wild, 6: Wild Draw 4) are always playable
    if (card.kind === 5 || card.kind === 6) return true;
    // Color matches active color
    if (activeColor && card.color === activeColor) return true;
    if (!topDiscard) return false;
    // Number card match
    if (card.kind === 1 && topDiscard.kind === 1 && card.value === topDiscard.value) return true;
    // Special action match (Skip: 2, Reverse: 3, Draw 2: 4)
    if (card.kind >= 2 && card.kind <= 4 && card.kind === topDiscard.kind) return true;
    return false;
}

// Table shockwave ripple on action card impact
function triggerShockwave() {
    if (!tableShockwave) return;
    tableShockwave.classList.remove('trigger-shockwave');
    void tableShockwave.offsetWidth; // re-trigger animation
    tableShockwave.classList.add('trigger-shockwave');
    setTimeout(() => {
        tableShockwave.classList.remove('trigger-shockwave');
    }, 700);
}

// Fullscreen celebratory confetti cannon on game victory
function launchConfetti() {
    if (!confettiCanvas) return;
    const ctx = confettiCanvas.getContext('2d');
    confettiCanvas.width = window.innerWidth;
    confettiCanvas.height = window.innerHeight;

    const particles = [];
    const colors = ['#ef4444', '#3b82f6', '#10b981', '#fbbf24', '#f43f5e', '#a855f7'];

    for (let i = 0; i < 140; i++) {
        particles.push({
            x: window.innerWidth / 2 + (Math.random() * 240 - 120),
            y: window.innerHeight / 2 + (Math.random() * 120 - 60),
            vx: (Math.random() - 0.5) * 16,
            vy: (Math.random() - 1.25) * 16,
            size: Math.random() * 8 + 4,
            color: colors[Math.floor(Math.random() * colors.length)],
            rotation: Math.random() * 360,
            rotationSpeed: (Math.random() - 0.5) * 12,
            opacity: 1
        });
    }

    if (confettiAnimationId) cancelAnimationFrame(confettiAnimationId);

    function render() {
        ctx.clearRect(0, 0, confettiCanvas.width, confettiCanvas.height);
        let active = 0;

        particles.forEach(p => {
            p.x += p.vx;
            p.y += p.vy;
            p.vy += 0.28; // gravity
            p.vx *= 0.99;
            p.rotation += p.rotationSpeed;
            if (p.y > window.innerHeight * 0.6) {
                p.opacity -= 0.015;
            }

            if (p.opacity > 0 && p.y < window.innerHeight + 50) {
                active++;
                ctx.save();
                ctx.translate(p.x, p.y);
                ctx.rotate((p.rotation * Math.PI) / 180);
                ctx.globalAlpha = Math.max(0, p.opacity);
                ctx.fillStyle = p.color;
                ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size * 1.4);
                ctx.restore();
            }
        });

        if (active > 0) {
            confettiAnimationId = requestAnimationFrame(render);
        } else {
            ctx.clearRect(0, 0, confettiCanvas.width, confettiCanvas.height);
        }
    }

    confettiAnimationId = requestAnimationFrame(render);
}

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
// GAME STATE RENDERER WITH 3D FLIGHT ANIMATIONS
// ==========================================================
function handleGameStateUpdate(snapshot) {
    const prev = state.prevSnapshot;
    state.gameSnapshot = snapshot;

    // Check if game is in Waiting phase (turnPlayerId == -1)
    if (snapshot.turnPlayerId === -1) {
        state.prevSnapshot = snapshot;
        renderWaitingLobby(snapshot);
        return;
    }

    // First time entering active game: render immediately without animation
    if (!prev || prev.turnPlayerId === -1) {
        state.prevSnapshot = snapshot;
        renderGameArena(snapshot);
        return;
    }

    // Determine what action took place between prev and current snapshot
    const actorId = prev.turnPlayerId;
    const oldDiscard = prev.discardPile;
    const newDiscard = snapshot.discardPile;

    const discardChanged = newDiscard && (!oldDiscard || 
        oldDiscard.color !== newDiscard.color || 
        oldDiscard.kind !== newDiscard.kind || 
        oldDiscard.value !== newDiscard.value);

    const oldDraw = prev.drawPile || 0;
    const newDraw = snapshot.drawPile || 0;
    const drawDiff = oldDraw - newDraw;

    // Case 1: Card was thrown on discard pile
    if (discardChanged && actorId !== -1) {
        animateCardPlay(actorId, newDiscard, () => {
            renderGameArena(snapshot);
        });
    }
    // Case 2: Card(s) drawn from draw pile
    else if (drawDiff > 0 && actorId !== -1) {
        animateCardDraw(actorId, Math.min(drawDiff, 4), () => {
            renderGameArena(snapshot);
        });
    }
    // Case 3: Other state changes (e.g. timeout / color change only)
    else {
        renderGameArena(snapshot);
    }

    state.prevSnapshot = snapshot;
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

    // Turn Timer Controller
    startTurnTimer(snapshot.turnDeadline, snapshot.turnDuration || 15);

    // Turn Banner & Action Prompt
    if (isMyTurn) {
        turnStatusBanner.className = "turn-status-banner my-turn";
        if (snapshot.hasDrawnCard) {
            turnStatusText.textContent = "★ Card Drawn! Play a valid card or PASS";
            btnPassTurn.classList.remove('hidden');
            btnDrawTurn.classList.add('hidden');
            if (drawPileHint) drawPileHint.textContent = "Click to pass turn";
        } else {
            turnStatusText.textContent = "★ YOUR TURN! Play a card or draw";
            btnPassTurn.classList.add('hidden');
            btnDrawTurn.classList.remove('hidden');
            if (drawPileHint) drawPileHint.textContent = "Click to draw";
        }
        playSound('turn');
    } else {
        const activePlayer = snapshot.players.find(p => p.playerId === snapshot.turnPlayerId);
        const name = activePlayer ? activePlayer.playerName : "Opponent";
        turnStatusBanner.className = "turn-status-banner";
        turnStatusText.textContent = `Waiting for ${name}'s turn...`;
        btnPassTurn.classList.add('hidden');
        btnDrawTurn.classList.remove('hidden');
        if (drawPileHint) drawPileHint.textContent = "Opponent's turn";
    }

    // Action Announcement Banner
    if (snapshot.lastAction && snapshot.lastAction !== "Waiting for players to join...") {
        actionBannerText.textContent = snapshot.lastAction;
        actionBanner.classList.remove('hidden');
        actionBanner.style.animation = 'none';
        void actionBanner.offsetWidth; // trigger reflow
        actionBanner.style.animation = 'popAction 0.5s cubic-bezier(0.34, 1.56, 0.64, 1)';
    }

    // Color Change Detection & Toast
    if (state.prevSnapshot && state.prevSnapshot.activeColor !== snapshot.activeColor && snapshot.activeColor !== 0) {
        const colorName = COLOR_NAMES[snapshot.activeColor] || "Color";
        showToast(`🎨 Table Color changed to ${colorName}!`);
    }

    // Render Opponents & UNO Challenge Badges
    opponentsArea.innerHTML = "";
    snapshot.players.forEach(p => {
        if (p.playerId !== state.playerId) {
            const oppChip = document.createElement('div');
            oppChip.className = `opponent-chip ${p.playerId === snapshot.turnPlayerId ? 'turn-active' : ''}`;
            oppChip.setAttribute('data-player-id', p.playerId);
            const initial = p.playerName.charAt(0).toUpperCase();
            oppChip.innerHTML = `
                <div class="opp-avatar">${initial}</div>
                <div class="opp-info">
                    <span class="opp-name">${p.playerName}</span>
                    <span class="opp-cards">🂠 ${p.handCardCount} cards</span>
                </div>
            `;

            // UNO Penalty & Vulnerability Badges
            if (p.handCardCount === 1) {
                if (p.hasSaidUno) {
                    const safeBadge = document.createElement('div');
                    safeBadge.className = 'uno-safe-badge';
                    safeBadge.textContent = 'UNO CALLED ✓';
                    oppChip.appendChild(safeBadge);
                } else if (p.isVulnerableToUno) {
                    const catchBtn = document.createElement('button');
                    catchBtn.className = 'btn-catch-uno';
                    catchBtn.textContent = '🚨 CATCH UNO!';
                    catchBtn.title = `Catch ${p.playerName} for not calling UNO! (3 Penalty Cards)`;
                    catchBtn.addEventListener('click', (e) => {
                        e.stopPropagation();
                        triggerCatchUno(p.playerId);
                    });
                    oppChip.appendChild(catchBtn);
                }
            }

            opponentsArea.appendChild(oppChip);
        }
    });

    // Check If I Should Call UNO (1 or 2 cards left and haven't called yet)
    const myPlayer = snapshot.players.find(p => p.playerId === state.playerId);
    if (myPlayer && myPlayer.handCardCount <= 2 && myPlayer.handCardCount >= 1 && !myPlayer.hasSaidUno) {
        btnSayUno.classList.remove('hidden');
    } else {
        btnSayUno.classList.add('hidden');
    }

    // Render Top Discard Card
    topCardContainer.innerHTML = "";
    if (snapshot.discardPile) {
        const cardElem = createCardElement(snapshot.discardPile, false);
        topCardContainer.appendChild(cardElem);
    }

    // Render Active Color & Ambient Table Aura
    const activeCol = snapshot.activeColor;
    activeColorDot.style.background = COLOR_HEX[activeCol] || '#ef4444';
    activeColorName.textContent = COLOR_NAMES[activeCol] || 'RED';

    if (tableSurface) {
        tableSurface.classList.remove('aura-1', 'aura-2', 'aura-3', 'aura-4');
        if (activeCol >= 1 && activeCol <= 4) {
            tableSurface.classList.add(`aura-${activeCol}`);
        }
    }

    // Direction Orbit Ring (snapshot.direction: 1 or -1)
    if (directionRing) {
        if (snapshot.direction === -1) {
            directionRing.classList.remove('clockwise');
            directionRing.classList.add('counter-clockwise');
        } else {
            directionRing.classList.remove('counter-clockwise');
            directionRing.classList.add('clockwise');
        }

        if (state.prevSnapshot && state.prevSnapshot.direction !== undefined && state.prevSnapshot.direction !== snapshot.direction) {
            directionRing.classList.remove('reverse-flash');
            void directionRing.offsetWidth;
            directionRing.classList.add('reverse-flash');
            setTimeout(() => directionRing.classList.remove('reverse-flash'), 800);
        }
    }

    // Render Draw Pile Count
    drawPileCountBadge.textContent = snapshot.drawPile || 0;

    // Render Player Hand with Parabolic Arc Fanning & Playable Highlighting
    const me = snapshot.players.find(p => p.playerId === state.playerId);
    myHandContainer.innerHTML = "";
    if (me && me.handCards) {
        myCardCountBadge.textContent = `${me.handCards.length} cards`;
        const totalCards = me.handCards.length;

        me.handCards.forEach((card, idx) => {
            const cardElem = createCardElement(card, true);
            const playable = isMyTurn && isCardPlayable(card, snapshot.discardPile, snapshot.activeColor);

            if (isMyTurn) {
                cardElem.classList.add(playable ? 'playable' : 'unplayable');
            }

            // Natural arced fan-out with parabolic vertical offset
            const offset = idx - (totalCards - 1) / 2;
            const rotateDeg = (offset * 3.5).toFixed(1);
            const translateY = (Math.abs(offset) * 4).toFixed(1);
            cardElem.style.transform = `translateY(${translateY}px) rotate(${rotateDeg}deg)`;

            cardElem.addEventListener('click', () => {
                if (!isMyTurn) {
                    playSound('error');
                    showToast("Not your turn!", true);
                    return;
                }
                if (!playable) {
                    playSound('error');
                    cardElem.classList.remove('card-shake');
                    void cardElem.offsetWidth; // re-trigger head shake
                    cardElem.classList.add('card-shake');
                    return;
                }
                state.lastClickedCardElement = cardElem;
                onCardClicked(card);
            });

            myHandContainer.appendChild(cardElem);
        });
    }

    // Check for Winner
    const winner = snapshot.players.find(p => p.handCardCount === 0);
    if (winner) {
        document.getElementById('winnerTitle').textContent = `${winner.playerName} Wins! 🎉`;
        document.getElementById('winnerSubtitle').textContent = winner.playerId === state.playerId ? "Congratulations, you won!" : "Better luck next time!";
        gameOverModal.classList.remove('hidden');
        launchConfetti();
    }
}

// ==========================================================
// 3D CARD ANIMATION ENGINES
// ==========================================================

// 1. Smooth Flight to Discard Pile with Exact Card Trajectory & Shockwave
function animateCardPlay(actorId, card, onComplete) {
    let sourceRect = null;

    if (actorId === state.playerId && state.lastClickedCardElement) {
        sourceRect = state.lastClickedCardElement.getBoundingClientRect();
        state.lastClickedCardElement.style.visibility = 'hidden';
    } else if (actorId === state.playerId) {
        const handEl = myHandContainer.querySelector('.card:last-child') || myHandContainer;
        sourceRect = handEl.getBoundingClientRect();
    } else {
        const oppEl = opponentsArea.querySelector(`[data-player-id="${actorId}"]`);
        if (oppEl) sourceRect = oppEl.getBoundingClientRect();
    }

    const targetEl = topCardContainer || document.getElementById('discardPile');
    if (!sourceRect || !targetEl) {
        state.lastClickedCardElement = null;
        if (onComplete) onComplete();
        return;
    }

    const targetRect = targetEl.getBoundingClientRect();

    // Create flying card element
    const flyingCard = createCardElement(card, false);
    flyingCard.classList.add('card-flying');

    // Initial position at exact source
    flyingCard.style.top = `${sourceRect.top}px`;
    flyingCard.style.left = `${sourceRect.left}px`;
    flyingCard.style.transform = actorId === state.playerId 
        ? 'translate(0, 0) scale(0.95) rotate(-6deg)' 
        : 'translate(0, 0) scale(0.65) rotate(20deg)';

    document.body.appendChild(flyingCard);
    playSound('card');

    // Trigger animation to target on next frame
    requestAnimationFrame(() => {
        const randomTilt = (Math.random() * 16 - 8).toFixed(1);
        flyingCard.style.top = `${targetRect.top}px`;
        flyingCard.style.left = `${targetRect.left}px`;
        flyingCard.style.transform = `translate(0, 0) scale(1) rotate(${randomTilt}deg)`;
    });

    setTimeout(() => {
        // Impact pulse on discard pile
        const discardPileEl = document.getElementById('discardPile');
        if (discardPileEl) {
            discardPileEl.classList.remove('discard-impact');
            void discardPileEl.offsetWidth; // re-trigger
            discardPileEl.classList.add('discard-impact');
        }

        // Action shockwave for special cards (Skip:2, Reverse:3, Draw2:4, WildDraw4:6)
        if (card.kind === 2 || card.kind === 3 || card.kind === 4 || card.kind === 6) {
            triggerShockwave();
        }

        flyingCard.remove();
        state.lastClickedCardElement = null;
        if (onComplete) onComplete();
    }, 420);
}

// 2. Smooth Flight from Draw Deck to Player Hand / Opponent
function animateCardDraw(actorId, count, onComplete) {
    const drawDeckEl = document.getElementById('drawPile');
    let targetEl = null;

    if (actorId === state.playerId) {
        targetEl = myHandContainer;
    } else {
        targetEl = opponentsArea.querySelector(`[data-player-id="${actorId}"]`);
    }

    if (!drawDeckEl || !targetEl) {
        if (onComplete) onComplete();
        return;
    }

    const deckRect = drawDeckEl.getBoundingClientRect();
    const targetRect = targetEl.getBoundingClientRect();

    const totalToAnimate = Math.min(count, 4);
    let completed = 0;

    for (let i = 0; i < totalToAnimate; i++) {
        setTimeout(() => {
            const flyingCard = document.createElement('div');
            flyingCard.className = 'card card-back card-flying';
            flyingCard.innerHTML = `
                <div class="card-back-labyrinth"></div>
                <div class="card-back-logo" style="font-size: 1.1rem; padding: 2px 8px;">UNO</div>
            `;

            flyingCard.style.top = `${deckRect.top}px`;
            flyingCard.style.left = `${deckRect.left}px`;
            flyingCard.style.transform = 'translate(0, 0) scale(1) rotate(0deg)';

            document.body.appendChild(flyingCard);
            playSound('card');

            requestAnimationFrame(() => {
                const targetScale = actorId === state.playerId ? '0.9' : '0.45';
                flyingCard.style.top = `${targetRect.top + (targetRect.height / 4)}px`;
                flyingCard.style.left = `${targetRect.left + (targetRect.width / 4)}px`;
                flyingCard.style.transform = `translate(0, 0) scale(${targetScale}) rotate(${i * 8}deg)`;
                flyingCard.style.opacity = '0.5';
            });

            setTimeout(() => {
                flyingCard.remove();
                if (actorId !== state.playerId && targetEl) {
                    targetEl.classList.remove('chip-pulse');
                    void targetEl.offsetWidth;
                    targetEl.classList.add('chip-pulse');
                }
                completed++;
                if (completed >= totalToAnimate && onComplete) {
                    onComplete();
                }
            }, 380);
        }, i * 90); // Stagger each card draw by 90ms
    }
}

// ==========================================================
// CARD COMPONENT BUILDER (UNO MINIMALISTA DARK EDITION)
// ==========================================================
function createCardElement(card, isInteractive = false) {
    const cardDiv = document.createElement('div');
    const colorClass = COLOR_CLASSES[card.color] || 'card-wild';
    cardDiv.className = `card ${colorClass}`;

    let cornerHtml = "";
    let centerHtml = "";

    // 1. Number Cards (0 - 9)
    if (card.kind === 1) {
        const val = card.value;
        const hasUnderline = (val === 6 || val === 9);
        cornerHtml = `<span>${val}</span>`;
        centerHtml = `
            <div class="card-center-val">
                <span>${val}</span>
                ${hasUnderline ? '<div class="number-underline"></div>' : ''}
            </div>
        `;
    }
    // 2. Action Cards (Skip, Reverse, Draw 2)
    else if (card.kind === 2) { // Skip
        cornerHtml = `<span>⊘</span>`;
        centerHtml = `<div class="card-center-val" style="font-size: 2.8rem;">⊘</div>`;
    } else if (card.kind === 3) { // Reverse
        cornerHtml = `<span>⇄</span>`;
        centerHtml = `<div class="card-center-val" style="font-size: 2.8rem;">⇄</div>`;
    } else if (card.kind === 4) { // Draw 2
        cornerHtml = `<span>+2</span>`;
        centerHtml = `<div class="card-center-val" style="font-size: 2.6rem; letter-spacing: -0.05em;">+2</div>`;
    }
    // 3. Wild Card (Overlapping Color Tiles from photo)
    else if (card.kind === 5) {
        cornerHtml = `<span class="wild-pill-badge">WILD</span>`;
        centerHtml = `
            <div class="wild-tiles-cluster">
                <div class="wild-tile tile-red"></div>
                <div class="wild-tile tile-blue"></div>
                <div class="wild-tile tile-yellow"></div>
                <div class="wild-tile tile-green"></div>
            </div>
        `;
    }
    // 4. Wild Draw 4 (+4 Overlapping Color Tiles)
    else if (card.kind === 6) {
        cornerHtml = `<span class="wild-pill-badge">+4</span>`;
        centerHtml = `
            <div class="wild-tiles-cluster">
                <div class="wild-tile tile-red"></div>
                <div class="wild-tile tile-blue"></div>
                <div class="wild-tile tile-yellow"></div>
                <div class="wild-tile tile-green"></div>
                <span class="wild-draw4-text">+4</span>
            </div>
        `;
    }

    cardDiv.innerHTML = `
        <svg class="card-arc-svg" viewBox="0 0 96 144" fill="none" preserveAspectRatio="none">
            <path class="card-arc-path" d="M 12 134 C 12 55, 38 18, 86 10" stroke-width="3" stroke-linecap="round"/>
            <path class="card-arc-path" d="M 84 10 C 84 89, 58 126, 10 134" stroke-width="1.2" opacity="0.3" stroke-linecap="round"/>
        </svg>
        <div class="card-corner corner-tl">${cornerHtml}</div>
        ${centerHtml}
        <div class="card-corner corner-br">${cornerHtml}</div>
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

// Turn Countdown Timer Engine
function startTurnTimer(deadlineMs, durationSec = 15) {
    if (turnTimerInterval) {
        clearInterval(turnTimerInterval);
        turnTimerInterval = null;
    }
    if (!deadlineMs || deadlineMs <= 0) {
        turnTimerBadge.classList.add('hidden');
        return;
    }
    turnTimerBadge.classList.remove('hidden');

    function tick() {
        const now = Date.now();
        const remainingMs = Math.max(0, deadlineMs - now);
        const remainingSec = Math.ceil(remainingMs / 1000);
        const pct = Math.min(100, Math.max(0, (remainingMs / (durationSec * 1000)) * 100));

        turnTimerBadge.textContent = `${remainingSec}s`;
        turnTimerBar.style.width = `${pct}%`;

        turnTimerBadge.classList.remove('warning', 'critical');
        turnTimerBar.classList.remove('warning', 'critical');

        if (remainingSec <= 3) {
            turnTimerBadge.classList.add('critical');
            turnTimerBar.classList.add('critical');
        } else if (remainingSec <= 6) {
            turnTimerBadge.classList.add('warning');
            turnTimerBar.classList.add('warning');
        }

        if (remainingMs <= 0) {
            clearInterval(turnTimerInterval);
            turnTimerInterval = null;
            turnTimerBadge.textContent = "0s";
            turnTimerBar.style.width = "0%";
        }
    }

    tick();
    turnTimerInterval = setInterval(tick, 100);
}

// Draw Card Button Click
function triggerDraw() {
    if (!state.gameSnapshot || state.gameSnapshot.turnPlayerId !== state.playerId) {
        showToast("Not your turn!", true);
        return;
    }
    if (state.gameSnapshot.hasDrawnCard) {
        triggerPass();
        return;
    }
    playSound('card');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({ action: "DRAW_CARD" }));
    }
}

// Pass Turn Button Click
function triggerPass() {
    if (!state.gameSnapshot || state.gameSnapshot.turnPlayerId !== state.playerId) {
        showToast("Not your turn!", true);
        return;
    }
    playSound('click');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({ action: "PASS_TURN" }));
    }
}

// Call UNO on self
function triggerSayUno() {
    playSound('turn');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({ action: "SAY_UNO" }));
    }
}

// Catch Opponent who forgot UNO
function triggerCatchUno(targetId) {
    playSound('error');
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
        state.ws.send(JSON.stringify({ action: "CATCH_UNO", targetId: targetId }));
    }
}

drawPile.addEventListener('click', triggerDraw);
btnDrawTurn.addEventListener('click', triggerDraw);
btnPassTurn.addEventListener('click', triggerPass);
btnSayUno.addEventListener('click', triggerSayUno);

btnReturnLobby.addEventListener('click', () => {
    window.location.href = "/";
});
