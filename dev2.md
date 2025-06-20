
```python
import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.utils.data import Dataset, DataLoader
import numpy as np
from transformers import BertModel, BertTokenizer, CLIPModel, CLIPProcessor
from PIL import Image
import pandas as pd

# 数据集类
class MultiModalAdDataset(Dataset):
    def __init__(self, data_path, image_dir, text_tokenizer, image_processor):
        self.data = pd.read_csv(data_path)
        self.image_dir = image_dir
        self.text_tokenizer = text_tokenizer
        self.image_processor = image_processor
        
    def __len__(self):
        return len(self.data)
    
    def __getitem__(self, idx):
        row = self.data.iloc[idx]
        
        # 用户历史序列
        user_history = eval(row['history_ads'])  # 假设是列表格式
        
        # 目标广告
        target_ad = row['target_ad']
        
        # 处理文本
        text_features = []
        for ad_id in user_history:
            text = self.get_ad_text(ad_id)
            text_encoded = self.text_tokenizer(text, truncation=True, 
                                              padding='max_length', 
                                              max_length=128,
                                              return_tensors='pt')
            text_features.append(text_encoded)
        
        # 处理图像
        image_features = []
        for ad_id in user_history:
            image_path = f"{self.image_dir}/{ad_id}.jpg"
            image = Image.open(image_path).convert('RGB')
            image_processed = self.image_processor(images=image, return_tensors='pt')
            image_features.append(image_processed)
        
        return {
            'user_id': row['user_id'],
            'history_ads': user_history,
            'text_features': text_features,
            'image_features': image_features,
            'target_ad': target_ad
        }
    
    def get_ad_text(self, ad_id):
        # 从数据库或文件中获取广告文本
        return f"Ad description for {ad_id}"

# 多模态编码器
class MultiModalEncoder(nn.Module):
    def __init__(self, hidden_dim=768):
        super().__init__()
        
        # 文本编码器
        self.text_encoder = BertModel.from_pretrained('bert-base-chinese')
        
        # 图像编码器
        self.image_encoder = CLIPModel.from_pretrained('openai/clip-vit-base-patch32').vision_model
        
        # 模态融合层
        self.fusion_layer = nn.MultiheadAttention(hidden_dim, num_heads=8)
        
        # 投影层
        self.text_projection = nn.Linear(768, hidden_dim)
        self.image_projection = nn.Linear(768, hidden_dim)
        
    def forward(self, text_inputs, image_inputs):
        # 编码文本
        text_outputs = self.text_encoder(**text_inputs)
        text_features = self.text_projection(text_outputs.pooler_output)
        
        # 编码图像
        image_outputs = self.image_encoder(**image_inputs)
        image_features = self.image_projection(image_outputs.pooler_output)
        
        # 融合多模态特征
        combined_features = torch.stack([text_features, image_features], dim=1)
        fused_features, _ = self.fusion_layer(combined_features, combined_features, combined_features)
        
        return fused_features.mean(dim=1)  # 池化

# 生成式推荐模型
class GenerativeRecommender(nn.Module):
    def __init__(self, num_ads, hidden_dim=768, num_layers=6):
        super().__init__()
        
        self.num_ads = num_ads
        self.hidden_dim = hidden_dim
        
        # 多模态编码器
        self.encoder = MultiModalEncoder(hidden_dim)
        
        # 广告嵌入
        self.ad_embedding = nn.Embedding(num_ads, hidden_dim)
        
        # Transformer解码器
        decoder_layer = nn.TransformerDecoderLayer(hidden_dim, nhead=8, 
                                                   dim_feedforward=2048,
                                                   batch_first=True)
        self.decoder = nn.TransformerDecoder(decoder_layer, num_layers=num_layers)
        
        # 生成头
        self.generation_head = nn.Linear(hidden_dim, num_ads)
        
        # 位置编码
        self.positional_encoding = nn.Parameter(torch.randn(1, 100, hidden_dim))
        
    def forward(self, history_sequence, text_features, image_features):
        batch_size = len(history_sequence)
        seq_len = len(history_sequence[0])
        
        # 编码历史序列的多模态特征
        encoded_features = []
        for i in range(seq_len):
            text_batch = {k: torch.stack([text_features[j][i][k].squeeze(0) 
                                         for j in range(batch_size)]) 
                         for k in text_features[0][0].keys()}
            image_batch = {k: torch.stack([image_features[j][i][k].squeeze(0) 
                                          for j in range(batch_size)]) 
                          for k in image_features[0][0].keys()}
            
            features = self.encoder(text_batch, image_batch)
            encoded_features.append(features)
        
        # 堆叠序列特征
        sequence_features = torch.stack(encoded_features, dim=1)
        
        # 添加位置编码
        sequence_features += self.positional_encoding[:, :seq_len, :]
        
        # 准备目标序列（用于训练时的teacher forcing）
        target_embeddings = self.ad_embedding(torch.tensor(history_sequence))
        
        # Transformer解码
        decoded = self.decoder(target_embeddings, sequence_features)
        
        # 生成概率分布
        logits = self.generation_head(decoded)
        
        return logits
    
    def generate_next(self, history_sequence, text_features, image_features, top_k=10):
        """生成下一个推荐的广告"""
        with torch.no_grad():
            logits = self.forward(history_sequence, text_features, image_features)
            
            # 获取最后一个时间步的预测
            next_ad_logits = logits[:, -1, :]
            
            # Top-k采样
            top_k_probs, top_k_indices = torch.topk(F.softmax(next_ad_logits, dim=-1), k=top_k)
            
            # 温度采样
            sampled_idx = torch.multinomial(top_k_probs, num_samples=1)
            next_ad = top_k_indices.gather(-1, sampled_idx)
            
            return next_ad

# 训练函数
def train_model(model, train_loader, val_loader, epochs=10, lr=1e-4):
    optimizer = torch.optim.AdamW(model.parameters(), lr=lr)
    criterion = nn.CrossEntropyLoss()
    
    for epoch in range(epochs):
        model.train()
        total_loss = 0
        
        for batch in train_loader:
            optimizer.zero_grad()
            
            # 准备输入
            history_ads = batch['history_ads']
            text_features = batch['text_features']
            image_features = batch['image_features']
            target_ads = batch['target_ad']
            
            # 前向传播
            logits = model(history_ads, text_features, image_features)
            
            # 计算损失（预测序列中的下一个广告）
            loss = criterion(logits[:, -1, :], target_ads)
            
            # 反向传播
            loss.backward()
            optimizer.step()
            
            total_loss += loss.item()
        
        # 验证
        model.eval()
        val_loss = evaluate_model(model, val_loader, criterion)
        
        print(f"Epoch {epoch+1}/{epochs}, Train Loss: {total_loss/len(train_loader):.4f}, "
              f"Val Loss: {val_loss:.4f}")

# 评估函数
def evaluate_model(model, data_loader, criterion):
    model.eval()
    total_loss = 0
    all_predictions = []
    all_targets = []
    
    with torch.no_grad():
        for batch in data_loader:
            history_ads = batch['history_ads']
            text_features = batch['text_features']
            image_features = batch['image_features']
            target_ads = batch['target_ad']
            
            logits = model(history_ads, text_features, image_features)
            loss = criterion(logits[:, -1, :], target_ads)
            total_loss += loss.item()
            
            # 收集预测结果
            predictions = torch.argmax(logits[:, -1, :], dim=-1)
            all_predictions.extend(predictions.cpu().numpy())
            all_targets.extend(target_ads.cpu().numpy())
    
    # 计算评估指标
    accuracy = calculate_accuracy(all_predictions, all_targets)
    recall_at_k = calculate_recall_at_k(all_predictions, all_targets, k=10)
    
    print(f"Accuracy: {accuracy:.4f}, Recall@10: {recall_at_k:.4f}")
    
    return total_loss / len(data_loader)

# 评估指标
def calculate_accuracy(predictions, targets):
    return np.mean(np.array(predictions) == np.array(targets))

def calculate_recall_at_k(predictions, targets, k=10):
    # 这里简化处理，实际应该基于概率分布计算
    recalls = []
    for pred, target in zip(predictions, targets):
        # 在实际实现中，应该从模型获取top-k预测
        recalls.append(1 if pred == target else 0)
    return np.mean(recalls)

# 主函数
def main():
    # 配置参数
    config = {
        'batch_size': 32,
        'epochs': 20,
        'learning_rate': 1e-4,
        'hidden_dim': 768,
        'num_ads': 10000,  # 广告总数
    }
    
    # 初始化分词器和处理器
    text_tokenizer = BertTokenizer.from_pretrained('bert-base-chinese')
    image_processor = CLIPProcessor.from_pretrained('openai/clip-vit-base-patch32')
    
    # 创建数据集
    train_dataset = MultiModalAdDataset('train.csv', 'images/', 
                                       text_tokenizer, image_processor)
    val_dataset = MultiModalAdDataset('val.csv', 'images/', 
                                     text_tokenizer, image_processor)
    
    # 创建数据加载器
    train_loader = DataLoader(train_dataset, batch_size=config['batch_size'], 
                            shuffle=True, num_workers=4)
    val_loader = DataLoader(val_dataset, batch_size=config['batch_size'], 
                          shuffle=False, num_workers=4)
    
    # 初始化模型
    model = GenerativeRecommender(num_ads=config['num_ads'], 
                                 hidden_dim=config['hidden_dim'])
    
    # 如果有GPU，使用GPU
    device = torch.device('cuda' if torch.cuda.is_available() else 'cpu')
    model = model.to(device)
    
    # 训练模型
    train_model(model, train_loader, val_loader, 
               epochs=config['epochs'], lr=config['learning_rate'])
    
    # 保存模型
    torch.save(model.state_dict(), 'generative_recommender.pth')

if __name__ == "__main__":
    main()
```


