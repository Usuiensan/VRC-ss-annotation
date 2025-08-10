import pyautogui
import time
import threading
import tkinter as tk
from tkinter import ttk
from PIL import ImageGrab
import cv2
import numpy as np
import datetime  # ログ用

class ClickerApp:
    def __init__(self, root):
        self.root = root
        self.root.title("自動釣りツール")
        self.root.geometry("500x400")  # ウィンドウサイズを設定
        # self.root.resizable(False, False)  # サイズ変更を禁止
        self.running = False
        self.count = 0
        self.last_click_time = None
        self.total_time = 0
        self.start_time = None  # 開始時間を記録
        self.status_var = tk.StringVar(value="待機中")  # ステータス表示用
        self.region_var = tk.StringVar(value="1600,900,320,80")  # 字幕領域の初期値を設定
        self.template_path = "template.png"  # テンプレート画像のパスを固定値として設定

        # 開始・停止ボタン
        button_frame = ttk.Frame(root)
        button_frame.grid(row=0, column=0, columnspan=3, pady=10)
        self.start_btn = ttk.Button(button_frame, text="開始", command=self.start)
        self.start_btn.pack(side="left", padx=10)
        self.stop_btn = ttk.Button(button_frame, text="停止", command=self.stop, state="disabled")
        self.stop_btn.pack(side="left", padx=10)

        # ステータス表示
        status_frame = ttk.Frame(root)
        status_frame.grid(row=1, column=0, columnspan=3, pady=10, sticky="ew")
        ttk.Label(status_frame, text="ステータス:").pack(side="left")
        self.status_label = ttk.Label(status_frame, textvariable=self.status_var)
        self.status_label.pack(side="left", padx=5)

        # 時間情報
        time_frame = ttk.LabelFrame(root, text="時間情報")
        time_frame.grid(row=2, column=0, columnspan=3, pady=10, padx=10, sticky="ew")
        ttk.Label(time_frame, text="経過時間:").grid(row=0, column=0, sticky="w")
        self.elapsed_time_label = ttk.Label(time_frame, text="0.0")
        self.elapsed_time_label.grid(row=0, column=1, padx=5)
        ttk.Label(time_frame, text="前回からの秒数:").grid(row=1, column=0, sticky="w")
        self.time_label = ttk.Label(time_frame, text="0.0")
        self.time_label.grid(row=1, column=1, padx=5)
        ttk.Label(time_frame, text="平均秒数:").grid(row=2, column=0, sticky="w")
        self.avg_time_label = ttk.Label(time_frame, text="0.0")
        self.avg_time_label.grid(row=2, column=1, padx=5)

        # 実行数カウンター（ウィンドウ下部に移動）
        counter_frame = ttk.Frame(root)
        counter_frame.grid(row=3, column=0, columnspan=3, pady=10, sticky="ew")
        ttk.Label(counter_frame, text="釣り上げた回数:").pack(side="left")
        self.counter_label = ttk.Label(counter_frame, text="0")
        self.counter_label.pack(side="left", padx=5)

        # ヘルプ/説明セクション
        help_frame = ttk.LabelFrame(root, text="ヘルプ")
        help_frame.grid(row=4, column=0, columnspan=3, pady=10, padx=10, sticky="ew")
        ttk.Label(help_frame, text="字幕領域とは、Minecraftの右下に表示される『浮きが沈む』という字幕が表示される、画面右下のエリアのことです。").pack(anchor="w")
        ttk.Label(help_frame, text="領域選択ボタンを使って設定してください。").pack(anchor="w")

        # 字幕領域設定
        region_frame = ttk.LabelFrame(root, text="字幕領域設定")
        region_frame.grid(row=5, column=0, columnspan=3, pady=10, padx=10, sticky="ew")
        ttk.Label(region_frame, text="字幕領域(x, y, w, h):").grid(row=0, column=0, sticky="w")
        ttk.Entry(region_frame, textvariable=self.region_var, width=20).grid(row=0, column=1, padx=5)
        ttk.Button(region_frame, text="領域選択", command=self.select_area).grid(row=0, column=2, padx=5)

    def template_match_loop(self):
        try:
            template_img = cv2.imread(self.template_path, 0)  # 固定値のテンプレート画像パスを使用
            if template_img is None:
                print("テンプレート画像が見つかりません")
                return
        except Exception as e:
            print("テンプレート画像読み込みエラー:", e)
            return

        while self.running:
            try:
                # self.status_var.set("魚を検知中")  # ステータス更新
                region_str = self.region_var.get()
                try:
                    x, y, w, h = map(int, region_str.split(","))
                except Exception:
                    print("字幕領域の指定が不正です。x,y,w,h をカンマ区切りで入力してください。")
                    time.sleep(1)
                    continue
                bbox = (x, y, x+w, y+h)
                img = ImageGrab.grab(bbox)
                img_gray = cv2.cvtColor(np.array(img), cv2.COLOR_BGR2GRAY)
                res = cv2.matchTemplate(img_gray, template_img, cv2.TM_CCOEFF_NORMED)
                threshold = 0.8
                loc = np.where(res >= threshold)
                if len(loc[0]) > 0:  # 浮きが沈んだのを検知
                    self.status_var.set("リールを巻きました")  # ステータス更新
                    current_time = time.time()
                    if self.last_click_time is not None:
                        elapsed_time = current_time - self.last_click_time
                        self.total_time += elapsed_time
                        avg_time = self.total_time / self.count
                        # 秒数ラベル更新
                        self.time_label.config(text=f"{elapsed_time:.2f}")
                        self.avg_time_label.config(text=f"{avg_time:.2f}")
                        # ログに記録
                        with open(self.log_file, "a") as f:
                            f.write(f"{datetime.datetime.now()}, {datetime.datetime.now().strftime('%H:%M:%S')}, {elapsed_time:.2f}, {avg_time:.2f}\n")
                        # DOS窓に出力
                        print(f"[釣り上げログ] 回数: {self.count + 1}, 時刻: {datetime.datetime.now().strftime('%H:%M:%S')}, 秒数: {elapsed_time:.2f}, 平均秒数: {avg_time:.2f}")
                    else:
                        # 初回ログ記録
                        self.total_time = 0
                        avg_time = 0
                        with open(self.log_file, "a") as f:
                            f.write(f"{datetime.datetime.now()}, {datetime.datetime.now().strftime('%H:%M:%S')}, 0.00, 0.00\n")
                        print(f"[釣り上げログ] 回数: 1")
                    self.last_click_time = current_time
                    pyautogui.click(button='right')  # 釣り糸を垂らす
                    self.count += 1
                    self.counter_label.config(text=str(self.count))
                    # 釣り上げた後3秒待ってから
                    for _ in range(60):  # 0.05秒 × 60 = 3秒
                        if not self.running:
                            break
                        time.sleep(0.05)
                    pyautogui.click(button='right')  # もう一度右クリック
                    self.last_click_time = time.time()  # 右クリック後に時間を記録
                else:
                    self.status_var.set("待機中")  # ステータス更新
                # 1/20秒ごとにチェック
                time.sleep(0.05)
            except Exception as e:
                self.status_var.set("エラー発生")  # ステータス更新
                print("テンプレートマッチングエラー:", e)
                time.sleep(1)

    def start(self):
        if not self.running:
            self.running = True
            self.start_time = time.time()  # 開始時間を記録
            self.start_btn.config(text="実行中", state="disabled")
            self.stop_btn.config(state="normal")
            self.thread = threading.Thread(target=self.template_match_loop, daemon=True)
            self.thread.start()
            self.update_elapsed_time()

    def stop(self):
        self.running = False
        self.start_btn.config(text="開始", state="normal")
        self.stop_btn.config(state="disabled")
        self.status_var.set("待機中")  # ステータス更新

    def update_elapsed_time(self):
        if self.running and self.start_time:
            elapsed_time = int(time.time() - self.start_time)  # 経過時間を整数に変更
            self.elapsed_time_label.config(text=f"{elapsed_time}")  # 小数点以下を削除
            self.root.after(1000, self.update_elapsed_time)  # 1秒ごとに更新

    def select_area(self):
        # サブウィンドウで画面全体を覆い、ドラッグで矩形選択
        sel_win = tk.Toplevel(self.root)
        sel_win.attributes("-fullscreen", True)
        sel_win.attributes("-alpha", 0.3)
        sel_win.attributes("-topmost", True)
        sel_win.config(bg="black")
        canvas = tk.Canvas(sel_win, cursor="cross", bg="black", highlightthickness=0)
        canvas.pack(fill="both", expand=True)

        self.start_x = self.start_y = self.end_x = self.end_y = None
        self.rect = None

        def on_mouse_down(event):
            self.start_x, self.start_y = event.x, event.y
            self.rect = canvas.create_rectangle(self.start_x, self.start_y, self.start_x, self.start_y, outline="red", width=2)

        def on_mouse_move(event):
            if self.rect:
                canvas.coords(self.rect, self.start_x, self.start_y, event.x, event.y)

        def on_mouse_up(event):
            self.end_x, self.end_y = event.x, event.y
            x1, y1 = min(self.start_x, self.end_x), min(self.start_y, self.end_y)
            x2, y2 = max(self.start_x, self.end_x), max(self.start_y, self.end_y)
            w, h = x2 - x1, y2 - y1
            self.region_var.set(f"{x1},{y1},{w},{h}")
            sel_win.destroy()

        canvas.bind("<ButtonPress-1>", on_mouse_down)
        canvas.bind("<B1-Motion>", on_mouse_move)
        canvas.bind("<ButtonRelease-1>", on_mouse_up)

if __name__ == "__main__":
    root = tk.Tk()
    app = ClickerApp(root)
    root.mainloop()