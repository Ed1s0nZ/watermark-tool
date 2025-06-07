/**
 * 文档水印工具 - 前端JavaScript
 * 处理用户界面交互、文件上传下载和API通信
 */

// 当DOM加载完成后执行
document.addEventListener('DOMContentLoaded', function() {
    // 初始化
    init();
});

/**
 * 初始化函数
 */
function init() {
    // 获取支持的文件类型
    loadSupportedTypes();
    
    // 初始化水印预览
    updateWatermarkPreview();
    
    // 初始化主题
    initTheme();
    
    // 添加事件监听器
    setupEventListeners();
}

/**
 * 初始化主题
 */
function initTheme() {
    // 检查本地存储中的主题设置
    const savedTheme = localStorage.getItem('theme');
    
    // 检查是否有保存的主题设置，或者用户系统偏好
    if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
        document.body.classList.add('dark-mode');
        document.getElementById('themeToggle').innerHTML = '<i class="fas fa-moon"></i>';
    } else {
        document.body.classList.remove('dark-mode');
        document.getElementById('themeToggle').innerHTML = '<i class="fas fa-sun"></i>';
    }
}

/**
 * 设置事件监听器
 */
function setupEventListeners() {
    // 主题切换
    document.getElementById('themeToggle').addEventListener('click', toggleTheme);
    
    // 选项卡切换
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            // 移除所有激活状态
            document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');
            
            // 设置当前选项卡为激活状态
            this.classList.add('active');
            const tabId = this.getAttribute('data-tab');
            document.getElementById(tabId).style.display = 'block';
        });
    });
    
    // 表单提交事件
    document.getElementById('addWatermarkForm').addEventListener('submit', handleAddWatermark);
    document.getElementById('extractWatermarkForm').addEventListener('submit', handleExtractWatermark);
    
    // 水印文本输入计数器
    const watermarkText = document.getElementById('watermarkText');
    watermarkText.addEventListener('input', function() {
        document.getElementById('charCount').textContent = this.value.length;
    });
    
    // 水印预览选项
    document.getElementById('previewSizeBtn').addEventListener('click', togglePreviewSize);
    document.getElementById('previewAngleBtn').addEventListener('click', togglePreviewAngle);
    
    // Hero按钮点击事件
    document.querySelectorAll('.hero-buttons a').forEach(btn => {
        btn.addEventListener('click', function(e) {
            e.preventDefault();
            const targetId = this.getAttribute('href').substring(1);
            
            // 切换到对应的选项卡
            document.querySelectorAll('.tab-btn').forEach(b => {
                if (b.getAttribute('data-tab') === targetId) {
                    b.click();
                }
            });
            
            // 平滑滚动到目标位置
            document.querySelector('.tabs-container').scrollIntoView({ 
                behavior: 'smooth' 
            });
        });
    });
}

/**
 * 切换主题
 */
function toggleTheme() {
    const body = document.body;
    const themeToggle = document.getElementById('themeToggle');
    
    if (body.classList.contains('dark-mode')) {
        body.classList.remove('dark-mode');
        themeToggle.innerHTML = '<i class="fas fa-sun"></i>';
        localStorage.setItem('theme', 'light');
    } else {
        body.classList.add('dark-mode');
        themeToggle.innerHTML = '<i class="fas fa-moon"></i>';
        localStorage.setItem('theme', 'dark');
    }
}

/**
 * 切换预览字号
 */
function togglePreviewSize() {
    const sizeBtn = document.getElementById('previewSizeBtn');
    const preview = document.getElementById('watermarkPreview');
    const currentSize = sizeBtn.getAttribute('data-size');
    
    if (currentSize === 'normal') {
        preview.style.fontSize = '2rem';
        sizeBtn.setAttribute('data-size', 'large');
    } else if (currentSize === 'large') {
        preview.style.fontSize = '2.5rem';
        sizeBtn.setAttribute('data-size', 'xlarge');
    } else {
        preview.style.fontSize = '1.5rem';
        sizeBtn.setAttribute('data-size', 'normal');
    }
}

/**
 * 切换预览角度
 */
function togglePreviewAngle() {
    const angleBtn = document.getElementById('previewAngleBtn');
    const preview = document.getElementById('watermarkPreview');
    const currentAngle = parseInt(angleBtn.getAttribute('data-angle'));
    
    // 计算新角度
    const newAngle = (currentAngle + 45) % 360;
    
    // 设置预览旋转
    preview.style.transform = `rotate(${newAngle}deg)`;
    angleBtn.setAttribute('data-angle', newAngle.toString());
}

/**
 * 加载支持的文件类型
 */
function loadSupportedTypes() {
    fetch('/api/supported-types')
        .then(response => response.json())
        .then(data => {
            if (data.types && Array.isArray(data.types)) {
                // 按字母顺序排序文件类型
                const sortedTypes = data.types.sort();
                
                // 更新主界面格式标签
                updateFormatTags(sortedTypes);
                
                // 更新上传区域的格式提示
                updateUploadAreaFormats(sortedTypes);
                
                // 更新页脚格式列表
                updateFooterFormats(sortedTypes);
            }
        })
        .catch(error => {
            console.error('获取支持的文件类型失败:', error);
            // 错误处理：显示默认格式
            const defaultTypes = ['pdf', 'docx', 'xlsx', 'pptx', 'jpg', 'png'];
            updateFormatTags(defaultTypes);
            updateUploadAreaFormats(defaultTypes);
            updateFooterFormats(defaultTypes);
        });
}

/**
 * 更新格式标签区域
 */
function updateFormatTags(types) {
    const typesContainer = document.getElementById('supportedTypes');
    typesContainer.innerHTML = '';
    
    types.forEach(type => {
        const tag = document.createElement('span');
        tag.className = 'format-tag';
        tag.textContent = type.toUpperCase();
        
        // 根据文件类型添加不同的图标
        const icon = document.createElement('i');
        icon.className = 'fas ';
        
        switch (type.toLowerCase()) {
            case 'pdf':
                icon.className += 'fa-file-pdf';
                break;
            case 'docx':
                icon.className += 'fa-file-word';
                break;
            case 'xlsx':
                icon.className += 'fa-file-excel';
                break;
            case 'pptx':
                icon.className += 'fa-file-powerpoint';
                break;
            case 'jpg':
            case 'jpeg':
                icon.className += 'fa-file-image';
                break;
            case 'png':
                icon.className += 'fa-file-image';
                break;
            case 'odt':
                icon.className += 'fa-file-alt';
                break;
            case 'rtf':
                icon.className += 'fa-file-alt';
                break;
            default:
                icon.className += 'fa-file';
                break;
        }
        
        tag.prepend(icon);
        icon.style.marginRight = '6px';
        typesContainer.appendChild(tag);
    });
}

/**
 * 更新上传区域的格式提示
 */
function updateUploadAreaFormats(types) {
    if (!types || types.length === 0) return;
    
    // 将类型转换为大写并格式化为易读的形式
    const formattedTypes = types.map(type => type.toUpperCase()).join('、');
    
    // 更新所有格式提示区域
    document.querySelectorAll('.upload-formats').forEach(el => {
        el.textContent = `支持 ${formattedTypes} 格式`;
    });
}

/**
 * 更新页脚格式列表
 */
function updateFooterFormats(types) {
    if (!types || types.length === 0) return;
    
    const footerFormats = document.getElementById('footerSupportedTypes');
    if (footerFormats) {
        footerFormats.textContent = types.map(type => type.toUpperCase()).join(', ');
    }
}

/**
 * 处理文件拖放
 */
function handleFileDrop(event, inputId) {
    event.preventDefault();
    event.currentTarget.classList.remove('dragover');
    
    const dt = event.dataTransfer;
    const files = dt.files;
    
    if (files.length > 0) {
        const fileInput = document.getElementById(inputId);
        fileInput.files = files;
        const changeEvent = new Event('change');
        fileInput.dispatchEvent(changeEvent);
    }
}

/**
 * 更新选择的文件名
 */
function updateFileName(input, containerId) {
    const container = document.getElementById(containerId);
    const emptyContainer = document.getElementById(containerId.replace('Selected', '') + 'Empty');
    
    if (input.files.length > 0) {
        const fileName = input.files[0].name;
        const fileNameElement = container.querySelector('.file-name');
        fileNameElement.textContent = fileName;
        
        // 获取文件图标
        const fileIcon = container.querySelector('.fas');
        
        // 根据文件类型设置图标
        const extension = fileName.split('.').pop().toLowerCase();
        fileIcon.className = 'fas ';
        
        switch (extension) {
            case 'pdf':
                fileIcon.className += 'fa-file-pdf';
                break;
            case 'docx':
                fileIcon.className += 'fa-file-word';
                break;
            case 'xlsx':
                fileIcon.className += 'fa-file-excel';
                break;
            case 'pptx':
                fileIcon.className += 'fa-file-powerpoint';
                break;
            case 'odt':
                fileIcon.className += 'fa-file-alt';
                break;
            case 'rtf':
                fileIcon.className += 'fa-file-alt';
                break;
            case 'jpg':
            case 'jpeg':
                fileIcon.className += 'fa-file-image';
                break;
            case 'png':
                fileIcon.className += 'fa-file-image';
                break;
            default:
                fileIcon.className += 'fa-file';
                break;
        }
        
        container.style.display = 'block';
        emptyContainer.style.display = 'none';
    } else {
        resetFileInput(input.id, containerId, emptyContainer.id);
    }
}

/**
 * 重置文件输入
 */
function resetFileInput(inputId, selectedContainerId, emptyContainerId) {
    document.getElementById(inputId).value = '';
    document.getElementById(selectedContainerId).style.display = 'none';
    document.getElementById(emptyContainerId).style.display = 'block';
}

/**
 * 更新水印预览
 */
function updateWatermarkPreview() {
    const watermarkText = document.getElementById('watermarkText').value;
    const preview = document.getElementById('watermarkPreview');
    
    // 更新预览文本
    if (watermarkText.trim()) {
        preview.textContent = watermarkText;
    } else {
        preview.textContent = '无隐水印预览';
    }
    
    // 更新字符计数
    document.getElementById('charCount').textContent = watermarkText.length;
}

/**
 * 处理添加水印表单提交
 */
function handleAddWatermark(event) {
    event.preventDefault();
    
    const fileInput = document.getElementById('addFileInput');
    const watermarkText = document.getElementById('watermarkText').value;
    const showTimestamp = document.getElementById('addShowTimestampCheckbox').checked;
    
    // 验证
    if (!fileInput.files.length) {
        showNotification('请选择文件', 'error');
        return;
    }
    
    if (!watermarkText.trim()) {
        showNotification('请输入水印文本', 'error');
        return;
    }
    
    // 准备表单数据
    const formData = new FormData();
    formData.append('file', fileInput.files[0]);
    formData.append('watermark', watermarkText);
    
    // 显示进度条
    const progressArea = document.getElementById('addProgress');
    progressArea.style.display = 'block';
    document.getElementById('addResult').style.display = 'none';
    
    // 禁用提交按钮
    const addBtn = document.getElementById('addWatermarkBtn');
    addBtn.disabled = true;
    addBtn.innerHTML = '<span class="icon"><i class="fas fa-spinner fa-spin"></i></span><span>处理中...</span>';
    
    // 发送请求
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/add-watermark', true);
    
    // 进度事件
    xhr.upload.onprogress = function(e) {
        if (e.lengthComputable) {
            const percentComplete = Math.round((e.loaded / e.total) * 100);
            updateProgress(percentComplete, progressArea);
        }
    };
    
    // 请求完成事件
    xhr.onload = function() {
        addBtn.disabled = false;
        addBtn.innerHTML = '<span class="icon"><i class="fas fa-stamp"></i></span><span>添加水印</span>';
        
        if (xhr.status === 200) {
            // 成功处理
            updateProgress(100, progressArea);
            
            // 创建URL并触发下载
            const blob = new Blob([xhr.response], { type: xhr.getResponseHeader('Content-Type') });
            const fileName = fileInput.files[0].name;
            const watermarkedFileName = 'watermarked_' + fileName;
            
            // 处理成功响应
            handleSuccessfulAddWatermark(blob, watermarkedFileName);
        } else {
            // 处理错误响应
            try {
                const response = JSON.parse(xhr.responseText);
                handleFailedRequest(response.error || '添加水印失败，请重试', xhr);
            } catch (e) {
                handleFailedRequest('添加水印失败，请重试', xhr);
            }
        }
    };
    
    // 请求错误事件
    xhr.onerror = function() {
        addBtn.disabled = false;
        addBtn.innerHTML = '<span class="icon"><i class="fas fa-stamp"></i></span><span>添加水印</span>';
        handleFailedRequest('网络错误，请检查您的连接', xhr);
    };
    
    // 设置响应类型为blob（用于文件下载）
    xhr.responseType = 'blob';
    
    // 发送请求
    xhr.send(formData);
}

/**
 * 处理成功添加水印
 */
function handleSuccessfulAddWatermark(blob, fileName) {
    const resultArea = document.getElementById('addResult');
    resultArea.style.display = 'block';
    
    // 创建下载链接
    const url = URL.createObjectURL(blob);
    
    // 显示成功消息和下载链接
    resultArea.innerHTML = `
        <div class="success-message">
            <div class="icon"><i class="fas fa-check-circle"></i></div>
            <div class="message">水印添加成功！您可以下载带有水印的文件。</div>
        </div>
        <a href="${url}" download="${fileName}" class="download-btn">
            <span class="icon"><i class="fas fa-download"></i></span>
            <span>下载文件</span>
        </a>
    `;
    
    // 自动滚动到结果区域
    resultArea.scrollIntoView({ behavior: 'smooth' });
}

/**
 * 处理提取水印表单提交
 */
function handleExtractWatermark(event) {
    event.preventDefault();
    
    const fileInput = document.getElementById('extractFileInput');
    const showTimestamp = document.getElementById('extractShowTimestampCheckbox').checked;
    
    // 验证
    if (!fileInput.files.length) {
        showNotification('请选择文件', 'error');
        return;
    }
    
    // 准备表单数据
    const formData = new FormData();
    formData.append('file', fileInput.files[0]);
    
    // 显示进度条
    const progressArea = document.getElementById('extractProgress');
    progressArea.style.display = 'block';
    document.getElementById('extractResult').style.display = 'none';
    
    // 禁用提交按钮
    const extractBtn = document.getElementById('extractWatermarkBtn');
    extractBtn.disabled = true;
    extractBtn.innerHTML = '<span class="icon"><i class="fas fa-spinner fa-spin"></i></span><span>处理中...</span>';
    
    // 发送请求
    fetch(`/api/extract-watermark?show_timestamp=${showTimestamp}`, {
        method: 'POST',
        body: formData
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(data => {
                throw new Error(data.error || '提取水印失败');
            });
        }
        return response.json();
    })
    .then(data => {
        // 更新进度
        updateProgress(100, progressArea);
        
        // 处理成功响应
        handleSuccessfulExtractWatermark(data);
    })
    .catch(error => {
        handleFailedRequest(error.message || '提取水印失败，请重试');
    })
    .finally(() => {
        extractBtn.disabled = false;
        extractBtn.innerHTML = '<span class="icon"><i class="fas fa-search"></i></span><span>提取水印</span>';
    });
}

/**
 * 处理成功提取水印
 */
function handleSuccessfulExtractWatermark(data) {
    const resultArea = document.getElementById('extractResult');
    resultArea.style.display = 'block';
    
    // 获取水印文本和时间戳
    const watermarkText = data.watermark;
    const timestamp = data.timestamp;
    
    // 构建HTML内容
    let htmlContent = `
        <div class="success-message">
            <div class="icon"><i class="fas fa-check-circle"></i></div>
            <div class="message">水印提取成功！</div>
        </div>
        <div class="watermark-content">
            <p class="watermark-content-title">水印内容</p>
            <p>${watermarkText}</p>
        </div>
    `;
    
    // 如果有时间戳，添加时间戳信息
    if (timestamp) {
        htmlContent += `
            <div class="watermark-timestamp">
                <p class="watermark-timestamp-title">水印添加时间</p>
                <p><i class="fas fa-calendar-alt"></i> ${timestamp}</p>
            </div>
        `;
    }
    
    // 显示结果
    resultArea.innerHTML = htmlContent;
    
    // 自动滚动到结果区域
    resultArea.scrollIntoView({ behavior: 'smooth' });
}

/**
 * 处理请求失败
 */
function handleFailedRequest(message, xhr) {
    // 确定要更新的结果区域
    const resultArea = xhr && xhr.responseURL.includes('add-watermark') 
        ? document.getElementById('addResult')
        : document.getElementById('extractResult');
    
    resultArea.style.display = 'block';
    
    // 显示错误消息
    resultArea.innerHTML = `
        <div class="error-message">
            <div class="icon"><i class="fas fa-exclamation-circle"></i></div>
            <div class="message">${message}</div>
        </div>
    `;
    
    // 自动滚动到结果区域
    resultArea.scrollIntoView({ behavior: 'smooth' });
}

/**
 * 更新进度条
 */
function updateProgress(percent, progressArea) {
    const progressFill = progressArea.querySelector('.progress-fill');
    const progressValue = progressArea.querySelector('.progress-value');
    
    progressFill.style.width = percent + '%';
    progressValue.textContent = percent + '%';
}

/**
 * 显示通知
 */
function showNotification(message, type) {
    // 创建通知元素
    const notification = document.createElement('div');
    notification.className = `notification ${type}`;
    notification.innerHTML = `
        <span class="icon">
            <i class="fas fa-${type === 'error' ? 'exclamation-circle' : 'check-circle'}"></i>
        </span>
        <span>${message}</span>
    `;
    
    // 添加到页面
    document.body.appendChild(notification);
    
    // 动画显示
    setTimeout(() => notification.classList.add('show'), 10);
    
    // 自动隐藏
    setTimeout(() => {
        notification.classList.remove('show');
        setTimeout(() => notification.remove(), 300);
    }, 3000);
} 
