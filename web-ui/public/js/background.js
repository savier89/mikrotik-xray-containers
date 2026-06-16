// Animated background with lightning effects
const canvas = document.getElementById('bgCanvas');
const ctx = canvas.getContext('2d');

let width, height;
let particles = [];
let lightning = [];

function resize() {
    width = canvas.width = window.innerWidth;
    height = canvas.height = window.innerHeight;
}

window.addEventListener('resize', resize);
resize();

// Particle class
class Particle {
    constructor() {
        this.reset();
    }
    
    reset() {
        this.x = Math.random() * width;
        this.y = Math.random() * height;
        this.vx = (Math.random() - 0.5) * 0.5;
        this.vy = (Math.random() - 0.5) * 0.5;
        this.radius = Math.random() * 2 + 0.5;
        this.opacity = Math.random() * 0.5 + 0.1;
    }
    
    update() {
        this.x += this.vx;
        this.y += this.vy;
        
        if (this.x < 0 || this.x > width) this.vx *= -1;
        if (this.y < 0 || this.y > height) this.vy *= -1;
    }
    
    draw() {
        ctx.beginPath();
        ctx.arc(this.x, this.y, this.radius, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(59, 130, 246, ${this.opacity})`;
        ctx.fill();
    }
}

// Lightning bolt class
class Lightning {
    constructor(x1, y1, x2, y2) {
        this.x1 = x1;
        this.y1 = y1;
        this.x2 = x2;
        this.y2 = y2;
        this.opacity = 1;
        this.segments = [];
        this.generateSegments();
    }
    
    generateSegments() {
        const segments = 10;
        const dx = (this.x2 - this.x1) / segments;
        const dy = (this.y2 - this.y1) / segments;
        
        for (let i = 0; i <= segments; i++) {
            const x = this.x1 + dx * i + (Math.random() - 0.5) * 50;
            const y = this.y1 + dy * i + (Math.random() - 0.5) * 50;
            this.segments.push({ x, y });
        }
    }
    
    update() {
        this.opacity *= 0.95;
    }
    
    draw() {
        if (this.opacity < 0.01) return;
        
        ctx.beginPath();
        ctx.moveTo(this.segments[0].x, this.segments[0].y);
        
        for (let i = 1; i < this.segments.length; i++) {
            ctx.lineTo(this.segments[i].x, this.segments[i].y);
        }
        
        ctx.strokeStyle = `rgba(59, 130, 246, ${this.opacity})`;
        ctx.lineWidth = 2;
        ctx.shadowColor = 'rgba(59, 130, 246, 0.5)';
        ctx.shadowBlur = 20;
        ctx.stroke();
        ctx.shadowBlur = 0;
    }
}

// Initialize particles
for (let i = 0; i < 100; i++) {
    particles.push(new Particle());
}

// Animation loop
function animate() {
    ctx.clearRect(0, 0, width, height);
    
    // Draw particles
    particles.forEach(p => {
        p.update();
        p.draw();
    });
    
    // Draw connections
    for (let i = 0; i < particles.length; i++) {
        for (let j = i + 1; j < particles.length; j++) {
            const dx = particles[i].x - particles[j].x;
            const dy = particles[i].y - particles[j].y;
            const distance = Math.sqrt(dx * dx + dy * dy);
            
            if (distance < 150) {
                ctx.beginPath();
                ctx.moveTo(particles[i].x, particles[i].y);
                ctx.lineTo(particles[j].x, particles[j].y);
                ctx.strokeStyle = `rgba(59, 130, 246, ${0.1 * (1 - distance / 150)})`;
                ctx.lineWidth = 0.5;
                ctx.stroke();
            }
        }
    }
    
    // Random lightning
    if (Math.random() < 0.005) {
        const x1 = Math.random() * width;
        const x2 = x1 + (Math.random() - 0.5) * 200;
        lightning.push(new Lightning(x1, 0, x2, height * 0.3));
    }
    
    // Update and draw lightning
    lightning = lightning.filter(l => l.opacity > 0.01);
    lightning.forEach(l => {
        l.update();
        l.draw();
    });
    
    requestAnimationFrame(animate);
}

animate();
