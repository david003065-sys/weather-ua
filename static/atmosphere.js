class Atmosphere {
    constructor() {
        this.canvas = document.getElementById('atmosphere-canvas');
        if (!this.canvas) return;
        this.ctx = this.canvas.getContext('2d');
        this.particles = [];
        this.weatherCode = 0;
        this.isNight = false;
        
        this.init();
        window.addEventListener('resize', () => this.init());
    }

    init() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
        this.createRain();
        this.animate();
    }

    // Создаем капли (Rain)
    createRain() {
        this.particles = [];
        for (let i = 0; i < 100; i++) {
            this.particles.push({
                x: Math.random() * this.canvas.width,
                y: Math.random() * this.canvas.height,
                length: Math.random() * 20 + 10,
                speed: Math.random() * 10 + 5,
                opacity: Math.random() * 0.3
            });
        }
    }

    update(code, isNight) {
        this.weatherCode = code;
        this.isNight = isNight;
    }

    drawRain() {
        // Коды дождя по WMO: 51-67, 80-82, 95+
        const isRainy = (this.weatherCode >= 51 && this.weatherCode <= 67) || 
                        (this.weatherCode >= 80 && this.weatherCode <= 82) || 
                        this.weatherCode >= 95;

        if (!isRainy) return;

        this.ctx.strokeStyle = 'rgba(255, 255, 255, 0.4)';
        this.ctx.lineWidth = 1;
        this.ctx.lineCap = 'round';

        this.particles.forEach(p => {
            this.ctx.beginPath();
            this.ctx.moveTo(p.x, p.y);
            this.ctx.lineTo(p.x, p.y + p.length);
            this.ctx.stroke();

            p.y += p.speed;
            if (p.y > this.canvas.height) {
                p.y = -p.length;
                p.x = Math.random() * this.canvas.width;
            }
        });
    }

    animate() {
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        
        // Здесь мы рисуем дождь поверх CSS-градиента
        this.drawRain();
        
        requestAnimationFrame(() => this.animate());
    }
}

// Инициализация
window.atmosphere = new Atmosphere();