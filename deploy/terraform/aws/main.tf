# -------------------------------------------------------------------
# Brockley -- AWS deployment
# Provisions: EKS Auto Mode + RDS PostgreSQL + ElastiCache Redis
# Then deploys the Brockley Helm chart into the cluster.
# -------------------------------------------------------------------

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      App     = "brockley"
      Managed = "terraform"
    }
  }
}

locals {
  tags = merge({
    App       = "brockley"
    Managed   = "terraform"
    Component = "brockley-platform"
  }, var.tags)

  azs = slice(data.aws_availability_zones.available.names, 0, 3)

  # Map size preset to RDS instance class
  db_instance_map = {
    starter     = "db.t4g.micro"
    standard    = "db.t4g.medium"
    performance = "db.r6g.large"
  }
  db_instance_class = var.database_instance_class != "" ? var.database_instance_class : local.db_instance_map[var.size]

  # Map size preset to ElastiCache node type
  redis_node_map = {
    starter     = "cache.t4g.micro"
    standard    = "cache.t4g.medium"
    performance = "cache.r6g.large"
  }
  redis_node = var.redis_node_type != "" ? var.redis_node_type : local.redis_node_map[var.size]
}

data "aws_availability_zones" "available" {
  state = "available"
}

# -------------------------------------------------------------------
# VPC
# -------------------------------------------------------------------

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(local.tags, { Name = "${var.name}-vpc" })
}

# Public subnets -- used by the load balancer
resource "aws_subnet" "public" {
  count             = 3
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index)
  availability_zone = local.azs[count.index]

  map_public_ip_on_launch = true

  tags = merge(local.tags, {
    Name                                    = "${var.name}-public-${local.azs[count.index]}"
    "kubernetes.io/role/elb"                = "1"
    "kubernetes.io/cluster/${var.name}-eks" = "shared"
  })
}

# Private subnets -- used by EKS nodes, RDS, ElastiCache
resource "aws_subnet" "private" {
  count             = 3
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index + 100)
  availability_zone = local.azs[count.index]

  tags = merge(local.tags, {
    Name                                    = "${var.name}-private-${local.azs[count.index]}"
    "kubernetes.io/role/internal-elb"       = "1"
    "kubernetes.io/cluster/${var.name}-eks" = "shared"
  })
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.main.id
  tags   = merge(local.tags, { Name = "${var.name}-igw" })
}

resource "aws_eip" "nat" {
  domain = "vpc"
  tags   = merge(local.tags, { Name = "${var.name}-nat-eip" })
}

resource "aws_nat_gateway" "nat" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id
  tags          = merge(local.tags, { Name = "${var.name}-nat" })

  depends_on = [aws_internet_gateway.igw]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  tags   = merge(local.tags, { Name = "${var.name}-public-rt" })
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.igw.id
}

resource "aws_route_table_association" "public" {
  count          = 3
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id
  tags   = merge(local.tags, { Name = "${var.name}-private-rt" })
}

resource "aws_route" "private_nat" {
  route_table_id         = aws_route_table.private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.nat.id
}

resource "aws_route_table_association" "private" {
  count          = 3
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# -------------------------------------------------------------------
# Security Groups
# -------------------------------------------------------------------

resource "aws_security_group" "rds" {
  name_prefix = "${var.name}-rds-"
  vpc_id      = aws_vpc.main.id
  description = "Allow PostgreSQL from EKS nodes"

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.main.vpc_config[0].cluster_security_group_id]
    description     = "PostgreSQL from EKS cluster"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, { Name = "${var.name}-rds-sg" })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_security_group" "redis" {
  name_prefix = "${var.name}-redis-"
  vpc_id      = aws_vpc.main.id
  description = "Allow Redis from EKS nodes"

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.main.vpc_config[0].cluster_security_group_id]
    description     = "Redis from EKS cluster"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, { Name = "${var.name}-redis-sg" })

  lifecycle {
    create_before_destroy = true
  }
}

# -------------------------------------------------------------------
# EKS Auto Mode
# -------------------------------------------------------------------

# IAM role for the EKS cluster
resource "aws_iam_role" "eks_cluster" {
  name = "${var.name}-eks-cluster"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "eks.amazonaws.com"
      }
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_iam_role_policy_attachment" "eks_compute_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSComputePolicy"
}

resource "aws_iam_role_policy_attachment" "eks_block_storage_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSBlockStoragePolicy"
}

resource "aws_iam_role_policy_attachment" "eks_lb_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSLoadBalancingPolicy"
}

resource "aws_iam_role_policy_attachment" "eks_networking_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSNetworkingPolicy"
}

resource "aws_eks_cluster" "main" {
  name     = "${var.name}-eks"
  role_arn = aws_iam_role.eks_cluster.arn
  version  = "1.31"

  vpc_config {
    subnet_ids              = aws_subnet.private[*].id
    endpoint_private_access = true
    endpoint_public_access  = true
  }

  # Auto Mode -- EKS manages compute, networking, and storage automatically.
  # No managed node groups needed.
  compute_config {
    enabled       = true
    node_pools    = ["general-purpose"]
    node_role_arn = aws_iam_role.eks_node.arn
  }

  kubernetes_network_config {
    elastic_load_balancing {
      enabled = true
    }
  }

  storage_config {
    block_storage {
      enabled = true
    }
  }

  access_config {
    authentication_mode = "API_AND_CONFIG_MAP"
  }

  tags = local.tags

  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy,
    aws_iam_role_policy_attachment.eks_compute_policy,
    aws_iam_role_policy_attachment.eks_block_storage_policy,
    aws_iam_role_policy_attachment.eks_lb_policy,
    aws_iam_role_policy_attachment.eks_networking_policy,
  ]
}

# IAM role for EKS Auto Mode nodes
resource "aws_iam_role" "eks_node" {
  name = "${var.name}-eks-node"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "eks_node_policy" {
  role       = aws_iam_role.eks_node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodeMinimalPolicy"
}

resource "aws_iam_role_policy_attachment" "ecr_pull" {
  role       = aws_iam_role.eks_node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly"
}

# -------------------------------------------------------------------
# RDS PostgreSQL
# -------------------------------------------------------------------

resource "aws_db_subnet_group" "postgres" {
  name       = "${var.name}-pg"
  subnet_ids = aws_subnet.private[*].id
  tags       = merge(local.tags, { Name = "${var.name}-pg-subnet-group" })
}

resource "random_password" "db_password" {
  length  = 32
  special = false
}

resource "aws_db_instance" "postgres" {
  identifier     = "${var.name}-pg"
  engine         = "postgres"
  engine_version = "16"
  instance_class = local.db_instance_class

  allocated_storage     = var.database_allocated_storage
  max_allocated_storage = var.database_allocated_storage * 5
  storage_encrypted     = true

  db_name  = "brockley"
  username = "brockley"
  password = random_password.db_password.result

  db_subnet_group_name   = aws_db_subnet_group.postgres.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  multi_az            = var.size != "starter"
  skip_final_snapshot = true
  deletion_protection = false

  backup_retention_period = var.size == "starter" ? 1 : 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  # Performance Insights free tier
  performance_insights_enabled = true

  tags = local.tags
}

# -------------------------------------------------------------------
# ElastiCache Redis
# -------------------------------------------------------------------

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.name}-redis"
  subnet_ids = aws_subnet.private[*].id
  tags       = local.tags
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${var.name}-redis"
  description          = "Brockley Redis -- asynq task queue and pub/sub"

  engine               = "redis"
  engine_version       = "7.1"
  node_type            = local.redis_node
  num_cache_clusters   = var.size == "starter" ? 1 : 2
  port                 = 6379
  parameter_group_name = "default.redis7"

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = false

  # Automatic failover requires > 1 node
  automatic_failover_enabled = var.size != "starter"

  tags = local.tags
}

# -------------------------------------------------------------------
# Kubernetes + Helm providers
# -------------------------------------------------------------------

data "aws_eks_cluster_auth" "main" {
  name = aws_eks_cluster.main.name
}

provider "kubernetes" {
  host                   = aws_eks_cluster.main.endpoint
  cluster_ca_certificate = base64decode(aws_eks_cluster.main.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.main.token
}

provider "helm" {
  kubernetes {
    host                   = aws_eks_cluster.main.endpoint
    cluster_ca_certificate = base64decode(aws_eks_cluster.main.certificate_authority[0].data)
    token                  = data.aws_eks_cluster_auth.main.token
  }
}

# -------------------------------------------------------------------
# Helm release
# -------------------------------------------------------------------

resource "helm_release" "brockley" {
  name      = var.name
  chart     = var.helm_chart_path
  namespace = "brockley"

  create_namespace = true
  wait             = true
  timeout          = 600

  # Apply the size-specific values file
  values = compact([
    file("${var.helm_chart_path}/values-${var.size}.yaml"),
    var.extra_helm_values,
  ])

  # Database connection
  set {
    name  = "postgresql.uri"
    value = "postgres://brockley:${random_password.db_password.result}@${aws_db_instance.postgres.endpoint}/brockley?sslmode=require"
  }

  # Redis connection
  set {
    name  = "redis.url"
    value = "redis://${aws_elasticache_replication_group.redis.primary_endpoint_address}:6379/0"
  }

  set {
    name  = "env"
    value = "production"
  }

  dynamic "set" {
    for_each = var.brockley_image_tag != "" ? [var.brockley_image_tag] : []
    content {
      name  = "server.image.tag"
      value = set.value
    }
  }

  depends_on = [
    aws_eks_cluster.main,
    aws_db_instance.postgres,
    aws_elasticache_replication_group.redis,
  ]
}
