import sys
import json
import threading
import queue
import tkinter as tk

# Forçar comunicação em UTF-8 no Windows
if hasattr(sys.stdin, 'reconfigure'):
    sys.stdin.reconfigure(encoding='utf-8')

q = queue.Queue()

def read_stdin():
    for line in sys.stdin:
        try:
            q.put(json.loads(line))
        except:
            pass
    # Se o laço for interrompido (EOF do Go), manda o pop-up morrer
    q.put({"action": "quit"})

threading.Thread(target=read_stdin, daemon=True).start()

root = tk.Tk()
root.overrideredirect(True)
root.attributes('-topmost', True)
root.attributes("-transparentcolor", "magenta")
root.config(bg='magenta')

# Frame para o popup de tradução
popup_frame = tk.Frame(root, bg='#1a1a24', highlightbackground='#ff9800', highlightthickness=1)

label_py = tk.Label(popup_frame, text="", fg="#ff9800", bg="#1a1a24", font=("Segoe UI", 14))
label_py.pack(pady=(8,0), padx=20)
label_hz = tk.Label(popup_frame, text="", fg="white", bg="#1a1a24", font=("Segoe UI", 32, "bold"))
label_hz.pack(padx=20)
label_sig = tk.Label(popup_frame, text="", fg="#cccccc", bg="#1a1a24", font=("Segoe UI", 12), wraplength=300, justify="center")
label_sig.pack(pady=(0,10), padx=20)

# Frame para o highlight box (borda)
highlight_frame = tk.Frame(root, bg='magenta', highlightbackground='#00ff00', highlightthickness=3)


# ----- "Mostrar pop-up de tudo": vários pop-ups simultâneos sem se invadirem -----
janelas_todos = []

# ----- Highlights azuis para palavras em estudo -----
janelas_estudos = []

def limpar_estudos():
    global janelas_estudos
    for janela in janelas_estudos:
        try:
            janela.destroy()
        except Exception:
            pass
    janelas_estudos = []

def limpar_todos():
    global janelas_todos
    for janela in janelas_todos:
        try:
            janela.destroy()
        except Exception:
            pass
    janelas_todos = []


def montar_conteudo(janela, item, escala):
    # Reconstrói o conteúdo da janela na escala pedida e devolve (largura, altura) requeridas.
    for filho in janela.winfo_children():
        filho.destroy()

    frame = tk.Frame(janela, bg='#1a1a24', highlightbackground='#ff9800', highlightthickness=1)
    frame.pack(fill='both', expand=True)

    fonte_py = ("Segoe UI", max(8, int(13 * escala)))
    fonte_hz = ("Segoe UI", max(11, int(26 * escala)), "bold")
    fonte_sig = ("Segoe UI", max(7, int(10 * escala)))
    wrap = max(80, int(240 * escala))
    pad = max(4, int(12 * escala))

    pinyin = item.get('pinyin', '')
    hanzi = item.get('hanzi', '')
    significados = item.get('significados', '')

    if pinyin:
        tk.Label(frame, text=pinyin, fg="#ff9800", bg="#1a1a24", font=fonte_py).pack(padx=pad, pady=(max(2, int(5 * escala)), 0))
    tk.Label(frame, text=hanzi, fg="white", bg="#1a1a24", font=fonte_hz).pack(padx=pad)
    if significados:
        # Trunca para manter o card compacto (quanto menor a escala, mais curto)
        limite = max(20, int(70 * escala))
        if len(significados) > limite:
            significados = significados[:limite - 1] + "…"
        tk.Label(frame, text=significados, fg="#cccccc", bg="#1a1a24", font=fonte_sig,
                 wraplength=wrap, justify="center").pack(padx=pad, pady=(0, max(3, int(8 * escala))))

    janela.update_idletasks()
    return frame.winfo_reqwidth(), frame.winfo_reqheight()


def colide(a, b, margem=4):
    ax0, ay0, ax1, ay1 = a
    bx0, by0, bx1, by1 = b
    return not (ax1 + margem <= bx0 or bx1 + margem <= ax0 or
                ay1 + margem <= by0 or by1 + margem <= ay0)


def achar_posicao(prefer_x, prefer_y, w, h, colocadas, sw, sh):
    # Tenta a posição preferida; se colidir, empilha verticalmente (cima/baixo) em passos.
    desloc = h + 8
    for mult in (0, -1, 1, -2, 2, -3, 3, -4, 4, -5, 5):
        x = max(0, min(prefer_x, sw - w))
        y = max(0, min(prefer_y + mult * desloc, sh - h))
        rect = (x, y, x + w, y + h)
        if not any(colide(rect, r) for r in colocadas):
            return x, y, rect
    return None


def mostrar_todos(itens):
    limpar_todos()
    sw = root.winfo_screenwidth()
    sh = root.winfo_screenheight()
    colocadas = []
    escalas = (1.0, 0.8, 0.65, 0.5)

    for item in itens:
        janela = tk.Toplevel(root)
        janela.overrideredirect(True)
        janela.attributes('-topmost', True)
        janela.config(bg='#1a1a24')

        x0 = item.get('x0', 0)
        y0 = item.get('y0', 0)
        x1 = item.get('x1', 0)
        y1 = item.get('y1', 0)
        centro_x = (x0 + x1) / 2

        colocado = False
        w = h = 0
        prefer_x = prefer_y = 0
        for escala in escalas:
            w, h = montar_conteudo(janela, item, escala)
            prefer_x = int(centro_x - w / 2)
            prefer_y = int(y0 - h - 6)
            if prefer_y < 0:
                prefer_y = int(y1 + 6)  # sem espaço acima: coloca abaixo da linha

            pos = achar_posicao(prefer_x, prefer_y, w, h, colocadas, sw, sh)
            if pos is not None:
                x, y, rect = pos
                janela.geometry(f"{w}x{h}+{x}+{y}")
                colocadas.append(rect)
                colocado = True
                break

        if not colocado:
            # Mesmo no menor tamanho não há espaço livre: coloca na preferida (aceita sobreposição)
            x = max(0, min(prefer_x, sw - w))
            y = max(0, min(prefer_y, sh - h))
            janela.geometry(f"{w}x{h}+{x}+{y}")
            colocadas.append((x, y, x + w, y + h))

        janelas_todos.append(janela)


def check_queue():
    try:
        while True:
            data = q.get_nowait()
            action = data.get("action")
            
            if action == "quit":
                limpar_todos()
                limpar_estudos()
                root.destroy()
                return
            elif action == "show_all":
                mostrar_todos(data.get("itens", []))
            elif action == "hide_all":
                limpar_todos()
            elif action == "hide":
                root.withdraw()
            elif action == "estudo_highlights":
                limpar_estudos()
                for box in (data.get("boxes") or []):
                    if len(box) != 4: continue
                    x0, y0, x1, y1 = box
                    w = int(x1 - x0)
                    h = int(y1 - y0)
                    if w <= 0: w = 1
                    if h <= 0: h = 1
                    
                    t = tk.Toplevel(root)
                    t.overrideredirect(True)
                    t.attributes('-topmost', True)
                    t.attributes("-transparentcolor", "magenta")
                    t.config(bg='magenta')
                    
                    f = tk.Frame(t, bg='magenta', highlightbackground='#2196f3', highlightthickness=3)
                    f.pack(fill='both', expand=True)
                    t.geometry(f"{w}x{h}+{int(x0)}+{int(y0)}")
                    janelas_estudos.append(t)
                
                
            elif action == "show":
                highlight_frame.place_forget()
                popup_frame.pack(fill='both', expand=True)
                
                label_py.config(text=data.get('pinyin', ''))
                label_hz.config(text=data.get('hanzi', ''))
                label_sig.config(text=data.get('significados', ''))
                
                root.update_idletasks()
                w = popup_frame.winfo_reqwidth()
                h = popup_frame.winfo_reqheight()
                
                x = data.get('x', 0)
                y = data.get('y', 0)
                
                final_x = int(x - (w / 2))
                final_y = int(y - h - 10)
                if final_y < 0:
                    final_y = int(y + 30)
                
                root.geometry(f"{w}x{h}+{final_x}+{final_y}")
                root.deiconify()
                
            elif action == "highlight":
                popup_frame.pack_forget()
                
                x0, y0, x1, y1 = data.get('x0'), data.get('y0'), data.get('x1'), data.get('y1')
                w = int(x1 - x0)
                h = int(y1 - y0)
                
                if w <= 0: w = 1
                if h <= 0: h = 1
                
                highlight_frame.place(x=0, y=0, width=w, height=h)
                root.geometry(f"{w}x{h}+{int(x0)}+{int(y0)}")
                root.deiconify()
                
    except queue.Empty:
        pass
    root.after(16, check_queue)

root.withdraw()
root.after(16, check_queue)
root.mainloop()
